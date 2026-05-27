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

app = FastAPI()
_models: dict[str, YOLO] = {}

# Fine-tune jobs: job_id -> {status, epoch, total_epochs, error}
_jobs: dict[str, dict] = {}
_jobs_lock = threading.Lock()


def get_model(name: str) -> YOLO:
    if name not in _models:
        _models[name] = YOLO(name + ".pt")
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
    status: str   # pending | running | done | error
    epoch: int
    total_epochs: int
    error: str = ""


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

        model.add_callback("on_train_epoch_end", EpochCallback())

        model.train(
            data=str(data_path),
            epochs=req.epochs,
            imgsz=640,
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
    except Exception as e:
        set_job(status="error", error=str(e))
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/analyze", response_model=AnalyzeResponse)
def analyze(req: AnalyzeRequest):
    if not os.path.isfile(req.path):
        raise HTTPException(status_code=404, detail=f"file not found: {req.path}")

    model = get_model(req.model)
    cap = cv2.VideoCapture(req.path)
    if not cap.isOpened():
        raise HTTPException(status_code=422, detail="cannot open video file")

    label_scores: dict[str, list[float]] = defaultdict(list)
    frame_counts: dict[str, int] = defaultdict(int)
    frame_skip = 30  # analyze every 30th frame (~1 frame/s at 30fps)
    frame_idx = 0

    try:
        while True:
            ret, frame = cap.read()
            if not ret:
                break
            if frame_idx % frame_skip == 0:
                results = model(frame, conf=req.confidence_threshold, verbose=False)
                for result in results:
                    for box in result.boxes:
                        label = model.names[int(box.cls)]
                        conf = float(box.conf)
                        label_scores[label].append(conf)
                        frame_counts[label] += 1
            frame_idx += 1
    finally:
        cap.release()

    detections = [
        Detection(
            label=label,
            confidence=round(max(scores), 4),
            frame_count=frame_counts[label],
        )
        for label, scores in sorted(label_scores.items(), key=lambda x: -max(x[1]))
    ]
    return AnalyzeResponse(detections=detections)


@app.post("/finetune", response_model=FinetuneResponse)
def finetune(req: FinetuneRequest):
    if not req.annotations:
        raise HTTPException(status_code=400, detail="annotations list is empty")

    job_id = str(uuid.uuid4())
    with _jobs_lock:
        _jobs[job_id] = {"status": "pending", "epoch": 0, "total_epochs": req.epochs, "error": ""}

    t = threading.Thread(target=_run_finetune, args=(job_id, req), daemon=True)
    t.start()
    return FinetuneResponse(job_id=job_id)


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
