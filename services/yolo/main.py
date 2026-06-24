from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from ultralytics import YOLO
import cv2
import os
import uuid
import threading
import shutil
import yaml
from collections import defaultdict
from pathlib import Path
import torch

app = FastAPI()
_models: dict[str, YOLO] = {}
MODEL_DIR = Path("/models")

# ── hardware detection ────────────────────────────────────────────────────────
def _detect_hardware() -> tuple[str, float]:
    """Returns (device, vram_gb). vram_gb=0 means CPU-only."""
    if torch.cuda.is_available():
        vram = torch.cuda.get_device_properties(0).total_memory / 1e9
        return "cuda", round(vram, 1)
    return "cpu", 0.0

_DEVICE, _VRAM_GB = _detect_hardware()

# VRAM requirements per size (GB) — finetune with batch=4
# Observed: yolo12l ~2.44GB, so 'l' fits in 4GB; 'x' is ~50% heavier than 'l'
_FINETUNE_VRAM = {"n": 1.0, "s": 1.5, "m": 2.0, "l": 3.0, "x": 5.0}
_INFER_VRAM    = {"n": 0.3, "s": 0.5, "m": 1.0, "l": 1.5, "x": 2.5}

_MODEL_GROUPS = [
    ("YOLOv8",  ["yolov8n",  "yolov8s",  "yolov8m",  "yolov8l",  "yolov8x"]),
    ("YOLO11",  ["yolo11n",  "yolo11s",  "yolo11m",  "yolo11l",  "yolo11x"]),
    ("YOLO12",  ["yolo12n",  "yolo12s",  "yolo12m",  "yolo12l",  "yolo12x"]),
]

def _build_model_list() -> list[dict]:
    result = []
    for group, names in _MODEL_GROUPS:
        for name in names:
            size = name[-1]  # n/s/m/l/x
            if _DEVICE == "cuda":
                can_infer    = _VRAM_GB >= _INFER_VRAM[size]
                can_finetune = _VRAM_GB >= _FINETUNE_VRAM[size]
            else:
                can_infer    = True   # CPU sempre consegue inferir (lento)
                can_finetune = False  # Fine-tuning em CPU não é prático
            result.append({
                "name": name,
                "group": group,
                "inference": can_infer,
                "finetune": can_finetune,
            })
    # Custom model — sempre disponível se existir
    if (MODEL_DIR / "custom.pt").exists():
        result.insert(0, {"name": "custom", "group": "Custom", "inference": True, "finetune": False})
    return result

# Fine-tune jobs: job_id -> {status, epoch, total_epochs, error}
_jobs: dict[str, dict] = {}
_jobs_lock = threading.Lock()
_cancel_events: dict[str, threading.Event] = {}


def get_model(name: str) -> YOLO:
    if name not in _models:
        custom_path = MODEL_DIR / f"{name}.pt"
        path = str(custom_path) if custom_path.exists() else f"{name}.pt"
        _models[name] = YOLO(path)
    return _models[name]


class AnalyzeRequest(BaseModel):
    path: str
    model: str = "yolov8n"
    confidence_threshold: float = 0.4


class Detection(BaseModel):
    label: str
    confidence: float
    frame_count: int


class AnalyzeResponse(BaseModel):
    detections: list[Detection]


class AnnotationItem(BaseModel):
    image_path: str
    label: str
    bbox_x: float  # pixels, absolute
    bbox_y: float
    bbox_w: float
    bbox_h: float


class FinetuneRequest(BaseModel):
    annotations: list[AnnotationItem]
    base_model: str = "yolov8n"
    epochs: int = 20


class FinetuneResponse(BaseModel):
    job_id: str


class FinetuneStatusResponse(BaseModel):
    status: str   # pending | running | cancelling | cancelled | done | error
    epoch: int
    total_epochs: int
    error: str = ""


# ── classificação de estado (state classification) ──────────────────────────
class ClassifyRequest(BaseModel):
    path: str
    model: str = "custom-cls"


class ClassPrediction(BaseModel):
    label: str
    prob: float


class ClassifyResponse(BaseModel):
    predictions: list[ClassPrediction]
    top: str


class ClassifySample(BaseModel):
    image_path: str
    label: str


class ClassifyTrainRequest(BaseModel):
    samples: list[ClassifySample]
    base_model: str = "yolov8n-cls"
    epochs: int = 20
    model: str = "custom-cls"  # nome do modelo de destino (um por classificador)


def _build_dataset(annotations: list[AnnotationItem], work_dir: Path) -> Path:
    """Build a YOLO-format dataset directory from annotation items."""
    images_dir = work_dir / "images" / "train"
    labels_dir = work_dir / "labels" / "train"
    images_dir.mkdir(parents=True)
    labels_dir.mkdir(parents=True)

    # Collect unique labels in stable order
    labels = list(dict.fromkeys(a.label for a in annotations))
    label_index = {l: i for i, l in enumerate(labels)}

    copied = 0
    for i, ann in enumerate(annotations):
        img = cv2.imread(ann.image_path)
        if img is None:
            print(f"[finetune] WARNING: cannot read image {ann.image_path!r}", flush=True)
            continue
        stem = f"{i:06d}"
        ext = Path(ann.image_path).suffix or ".jpg"
        shutil.copy2(ann.image_path, images_dir / f"{stem}{ext}")
        # bbox stored as normalized (0-1) coordinates relative to the displayed image.
        # These are already valid YOLO center-x, center-y, width, height in [0,1].
        cx = max(0.0, min(1.0, ann.bbox_x + ann.bbox_w / 2))
        cy = max(0.0, min(1.0, ann.bbox_y + ann.bbox_h / 2))
        nw = max(0.001, min(1.0, ann.bbox_w))
        nh = max(0.001, min(1.0, ann.bbox_h))
        with open(labels_dir / f"{stem}.txt", "w") as f:
            f.write(f"{label_index[ann.label]} {cx:.6f} {cy:.6f} {nw:.6f} {nh:.6f}\n")
        copied += 1

    if copied == 0:
        first_path = annotations[0].image_path if annotations else "N/A"
        raise ValueError(
            f"No images could be read from {len(annotations)} annotation(s). "
            f"First path attempted: {first_path!r}"
        )

    # Use absolute paths to avoid YOLO path-resolution ambiguity
    data_yaml = {
        "train": str(images_dir),
        "val": str(images_dir),
        "nc": len(labels),
        "names": labels,
    }
    data_path = work_dir / "data.yaml"
    with open(data_path, "w") as f:
        yaml.dump(data_yaml, f)

    print(f"[finetune] dataset ready: {copied} image(s), classes={labels}", flush=True)
    return data_path


def _run_finetune(job_id: str, req: FinetuneRequest):
    work_dir = Path(f"/tmp/finetune_{job_id}")
    work_dir.mkdir(parents=True, exist_ok=True)
    cancel_event = _cancel_events[job_id]

    def set_job(**kwargs):
        with _jobs_lock:
            _jobs[job_id].update(kwargs)

    try:
        set_job(status="running")
        data_path = _build_dataset(req.annotations, work_dir)

        model = YOLO(req.base_model + ".pt")

        class EpochCallback:
            def __call__(self, trainer):
                with _jobs_lock:
                    _jobs[job_id]["epoch"] = trainer.epoch + 1
                if cancel_event.is_set():
                    raise StopIteration("cancelled")

        model.add_callback("on_train_epoch_end", EpochCallback())

        model.train(
            data=str(data_path),
            epochs=req.epochs,
            imgsz=640,
            batch=4,
            project=str(work_dir / "runs"),
            name="train",
            exist_ok=True,
            verbose=False,
        )

        # Save fine-tuned model to shared location
        best = work_dir / "runs" / "train" / "weights" / "best.pt"
        if best.exists():
            dest = Path("/models/custom.pt")
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(best, dest)
            # Invalidate cached model so next analyze uses the new weights
            _models.pop("custom", None)

        set_job(status="done")
    except StopIteration:
        set_job(status="cancelled")
    except Exception as e:
        set_job(status="error", error=str(e))
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)
        with _jobs_lock:
            _cancel_events.pop(job_id, None)


# Modelos de classificação disponíveis (ultralytics expõe variantes -cls p/ v8 e 11).
_CLS_GROUPS = [
    ("YOLOv8-cls", ["yolov8n-cls", "yolov8s-cls", "yolov8m-cls", "yolov8l-cls", "yolov8x-cls"]),
    ("YOLO11-cls", ["yolo11n-cls", "yolo11s-cls", "yolo11m-cls", "yolo11l-cls", "yolo11x-cls"]),
]


def _cls_size(model_name: str) -> str:
    """n/s/m/l/x a partir de um nome como 'yolov8n-cls'."""
    return model_name.replace("-cls", "")[-1:]


def _build_classify_model_list() -> list[dict]:
    result = []
    for group, names in _CLS_GROUPS:
        for name in names:
            size = _cls_size(name)
            if _DEVICE == "cuda":
                can_infer = _VRAM_GB >= _INFER_VRAM[size]
                can_train = _VRAM_GB >= _FINETUNE_VRAM[size]
            else:
                can_infer = True              # CPU sempre infere (lento)
                can_train = size in ("n", "s")  # classify n/s treina rápido até em CPU
            result.append({"name": name, "group": group, "inference": can_infer, "finetune": can_train})
    if (MODEL_DIR / "custom-cls.pt").exists():
        result.insert(0, {"name": "custom-cls", "group": "Custom", "inference": True, "finetune": False})
    return result


def _build_classify_dataset(samples: list[ClassifySample], work_dir: Path) -> tuple[Path, list[str]]:
    """Monta um dataset de classificação ultralytics: <root>/{train,val}/<classe>/*.

    Exige ≥2 classes. Levanta ValueError se < 2 classes ou nenhuma imagem legível.
    """
    labels = list(dict.fromkeys(s.label for s in samples))
    if len(labels) < 2:
        raise ValueError(f"classification needs at least 2 classes, got {len(labels)}")
    for split in ("train", "val"):
        for label in labels:
            (work_dir / split / label).mkdir(parents=True, exist_ok=True)
    copied = 0
    for i, s in enumerate(samples):
        src = Path(s.image_path)
        if not src.is_file():
            print(f"[classify] WARNING: cannot read image {s.image_path!r}", flush=True)
            continue
        ext = src.suffix or ".jpg"
        for split in ("train", "val"):
            shutil.copy2(src, work_dir / split / s.label / f"{i:06d}{ext}")
        copied += 1
    if copied == 0:
        raise ValueError(f"no images could be read from {len(samples)} sample(s)")
    print(f"[classify] dataset ready: {copied} image(s), classes={labels}", flush=True)
    return work_dir, labels


def _run_classify_train(job_id: str, req: ClassifyTrainRequest):
    work_dir = Path(f"/tmp/classify_{job_id}")
    work_dir.mkdir(parents=True, exist_ok=True)
    cancel_event = _cancel_events[job_id]

    def set_job(**kwargs):
        with _jobs_lock:
            _jobs[job_id].update(kwargs)

    try:
        set_job(status="running")
        root, _labels = _build_classify_dataset(req.samples, work_dir / "data")

        model = YOLO(req.base_model + ".pt")

        class EpochCallback:
            def __call__(self, trainer):
                with _jobs_lock:
                    _jobs[job_id]["epoch"] = trainer.epoch + 1
                if cancel_event.is_set():
                    raise StopIteration("cancelled")

        model.add_callback("on_train_epoch_end", EpochCallback())
        model.train(
            data=str(root),
            epochs=req.epochs,
            imgsz=224,
            batch=16,
            # Sem espelhamento horizontal: classes direcionais (ex.: pessoa
            # entrando vs saindo) seriam corrompidas — uma imagem espelhada de
            # "entrando" vira visualmente "saindo", contradizendo o rótulo.
            fliplr=0.0,
            project=str(work_dir / "runs"),
            name="train",
            exist_ok=True,
            verbose=False,
        )

        best = work_dir / "runs" / "train" / "weights" / "best.pt"
        if best.exists():
            dest = MODEL_DIR / f"{req.model}.pt"
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(best, dest)
            _models.pop(req.model, None)  # invalida o cache desse modelo

        set_job(status="done")
    except StopIteration:
        set_job(status="cancelled")
    except Exception as e:
        set_job(status="error", error=str(e))
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)
        with _jobs_lock:
            _cancel_events.pop(job_id, None)


def _analyze_one(model: YOLO, cap, conf_threshold: float):
    label_scores: dict[str, list[float]] = defaultdict(list)
    frame_counts: dict[str, int] = defaultdict(int)
    cap.set(cv2.CAP_PROP_POS_FRAMES, 0)
    frame_idx = 0
    while True:
        ret, frame = cap.read()
        if not ret:
            break
        if frame_idx % 30 == 0:
            for result in model(frame, conf=conf_threshold, verbose=False):
                for box in result.boxes:
                    label = model.names[int(box.cls)]
                    label_scores[label].append(float(box.conf))
                    frame_counts[label] += 1
        frame_idx += 1
    return label_scores, frame_counts


@app.get("/health")
def health():
    return {"status": "ok", "device": _DEVICE, "vram_gb": _VRAM_GB}


@app.get("/models")
def list_models():
    return {
        "device": _DEVICE,
        "vram_gb": _VRAM_GB,
        "models": _build_model_list(),
    }


@app.post("/analyze", response_model=AnalyzeResponse)
def analyze(req: AnalyzeRequest):
    if not os.path.isfile(req.path):
        raise HTTPException(status_code=404, detail=f"file not found: {req.path}")

    cap = cv2.VideoCapture(req.path)
    if not cap.isOpened():
        raise HTTPException(status_code=422, detail="cannot open video file")

    merged_scores: dict[str, list[float]] = defaultdict(list)
    merged_counts: dict[str, int] = defaultdict(int)

    try:
        for name in req.model.split("+"):
            name = name.strip()
            scores, counts = _analyze_one(get_model(name), cap, req.confidence_threshold)
            for label, s in scores.items():
                merged_scores[label].extend(s)
                merged_counts[label] += counts[label]
    finally:
        cap.release()

    detections = [
        Detection(
            label=label,
            confidence=round(max(scores), 4),
            frame_count=merged_counts[label],
        )
        for label, scores in sorted(merged_scores.items(), key=lambda x: -max(x[1]))
    ]
    return AnalyzeResponse(detections=detections)


@app.post("/finetune", response_model=FinetuneResponse)
def finetune(req: FinetuneRequest):
    if not req.annotations:
        raise HTTPException(status_code=400, detail="annotations list is empty")

    job_id = str(uuid.uuid4())
    cancel_event = threading.Event()
    with _jobs_lock:
        _jobs[job_id] = {"status": "pending", "epoch": 0, "total_epochs": req.epochs, "error": ""}
        _cancel_events[job_id] = cancel_event

    t = threading.Thread(target=_run_finetune, args=(job_id, req), daemon=True)
    t.start()
    return FinetuneResponse(job_id=job_id)


@app.delete("/finetune/{job_id}", status_code=204)
def cancel_finetune(job_id: str):
    with _jobs_lock:
        job = _jobs.get(job_id)
        event = _cancel_events.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    if event is not None:
        event.set()
        with _jobs_lock:
            if _jobs[job_id]["status"] in ("pending", "running"):
                _jobs[job_id]["status"] = "cancelling"


@app.get("/finetune/status/{job_id}", response_model=FinetuneStatusResponse)
def finetune_status(job_id: str):
    with _jobs_lock:
        job = _jobs.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    return FinetuneStatusResponse(
        status=job["status"],
        epoch=job["epoch"],
        total_epochs=job["total_epochs"],
        error=job.get("error", ""),
    )


@app.get("/classify/models")
def list_classify_models():
    return {
        "device": _DEVICE,
        "vram_gb": _VRAM_GB,
        "models": _build_classify_model_list(),
    }


@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    if not os.path.isfile(req.path):
        raise HTTPException(status_code=404, detail=f"file not found: {req.path}")
    # Modelo ainda não treinado → 404 limpo (o runner ignora) em vez de o ultralytics
    # estourar FileNotFoundError tentando carregar um .pt inexistente.
    if not (MODEL_DIR / f"{req.model}.pt").exists():
        raise HTTPException(status_code=404, detail=f"model '{req.model}' not trained yet")

    model = get_model(req.model)
    results = model.predict(req.path, verbose=False)
    r = results[0]
    raw = r.probs.data if hasattr(r.probs, "data") else r.probs
    probs = [float(p) for p in raw]
    names = model.names
    preds = [ClassPrediction(label=names[i], prob=round(p, 4)) for i, p in enumerate(probs)]
    preds.sort(key=lambda p: -p.prob)
    return ClassifyResponse(predictions=preds, top=preds[0].label if preds else "")


@app.post("/classify/train", response_model=FinetuneResponse)
def classify_train(req: ClassifyTrainRequest):
    if not req.samples:
        raise HTTPException(status_code=400, detail="samples list is empty")
    size = _cls_size(req.base_model)
    if size in ("l", "x"):
        raise HTTPException(status_code=400, detail=f"model size {size!r} too large for training (use n/s/m)")

    job_id = str(uuid.uuid4())
    with _jobs_lock:
        _jobs[job_id] = {"status": "pending", "epoch": 0, "total_epochs": req.epochs, "error": ""}
        _cancel_events[job_id] = threading.Event()

    t = threading.Thread(target=_run_classify_train, args=(job_id, req), daemon=True)
    t.start()
    return FinetuneResponse(job_id=job_id)
