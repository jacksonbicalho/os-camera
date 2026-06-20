"""Testes do serviço YOLO.

As deps pesadas (torch/ultralytics/cv2) são stubadas via sys.modules ANTES de
importar `main`, então os testes rodam numa imagem Python slim, sem GPU nem
torch/ultralytics instalados — só fastapi/pydantic/pyyaml/httpx/pytest.
"""
import sys
import time
import types
from pathlib import Path

import pytest


# ── stubs das deps pesadas ──────────────────────────────────────────────────
class _FakeProbs:
    def __init__(self, data):
        self.data = data


class _FakeResult:
    def __init__(self, probs):
        self.probs = _FakeProbs(probs)


class FakeYOLO:
    """Mímica mínima do ultralytics.YOLO para classificação."""
    last_train_kwargs = None

    def __init__(self, path):
        self.path = path
        self.names = {0: "aberto", 1: "fechado"}

    def predict(self, source, **kw):
        return [_FakeResult([0.2, 0.8])]  # → top1 = índice 1 = "fechado"

    def train(self, **kw):
        FakeYOLO.last_train_kwargs = kw
        # simula o best.pt para o worker conseguir salvar o modelo
        best = Path(kw["project"]) / kw["name"] / "weights" / "best.pt"
        best.parent.mkdir(parents=True, exist_ok=True)
        best.write_bytes(b"trained")

    def add_callback(self, *a, **k):
        pass


def _install_stubs():
    torch = types.ModuleType("torch")
    torch.cuda = types.SimpleNamespace(is_available=lambda: False)
    sys.modules["torch"] = torch
    sys.modules["cv2"] = types.ModuleType("cv2")
    ultra = types.ModuleType("ultralytics")
    ultra.YOLO = FakeYOLO
    sys.modules["ultralytics"] = ultra


_install_stubs()

import main  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402

client = TestClient(main.app)


# ── /classify (inferência) ──────────────────────────────────────────────────
def test_classify_returns_probabilities(tmp_path, monkeypatch):
    monkeypatch.setattr(main, "MODEL_DIR", tmp_path)
    (tmp_path / "custom-cls.pt").write_bytes(b"model")  # modelo treinado existe
    img = tmp_path / "crop.jpg"
    img.write_bytes(b"fake")
    resp = client.post("/classify", json={"path": str(img), "model": "custom-cls"})
    assert resp.status_code == 200
    body = resp.json()
    labels = {p["label"]: p["prob"] for p in body["predictions"]}
    assert labels == {"aberto": 0.2, "fechado": 0.8}
    assert body["top"] == "fechado"


def test_classify_missing_file_404(tmp_path, monkeypatch):
    monkeypatch.setattr(main, "MODEL_DIR", tmp_path)
    (tmp_path / "custom-cls.pt").write_bytes(b"m")
    resp = client.post("/classify", json={"path": "/nope.jpg", "model": "custom-cls"})
    assert resp.status_code == 404


def test_classify_untrained_model_404(tmp_path, monkeypatch):
    # modelo ainda não treinado → 404 limpo, sem estourar o ultralytics
    monkeypatch.setattr(main, "MODEL_DIR", tmp_path)
    img = tmp_path / "crop.jpg"
    img.write_bytes(b"fake")
    resp = client.post("/classify", json={"path": str(img), "model": "custom-cls-99"})
    assert resp.status_code == 404


# ── /classify/models ────────────────────────────────────────────────────────
def test_classify_models_lists_cls_models():
    resp = client.get("/classify/models")
    assert resp.status_code == 200
    names = [m["name"] for m in resp.json()["models"]]
    assert "yolov8n-cls" in names


# ── dataset builder (pastas por classe) ─────────────────────────────────────
def test_build_classify_dataset_creates_class_folders(tmp_path):
    src = tmp_path / "src"
    src.mkdir()
    samples = []
    for i, label in enumerate(["aberto", "fechado", "aberto"]):
        f = src / f"{i}.jpg"
        f.write_bytes(b"x")
        samples.append(main.ClassifySample(image_path=str(f), label=label))
    root, labels = main._build_classify_dataset(samples, tmp_path / "ds")
    assert set(labels) == {"aberto", "fechado"}
    # train/ e val/ com subpasta por classe
    for split in ("train", "val"):
        assert (root / split / "aberto").is_dir()
        assert (root / split / "fechado").is_dir()
    # 3 imagens distribuídas nas pastas de train
    train_imgs = list((root / "train").rglob("*.jpg"))
    assert len(train_imgs) == 3


def test_build_classify_dataset_requires_two_classes(tmp_path):
    src = tmp_path / "s"
    src.mkdir()
    f = src / "a.jpg"
    f.write_bytes(b"x")
    samples = [main.ClassifySample(image_path=str(f), label="aberto")]
    with pytest.raises(ValueError):
        main._build_classify_dataset(samples, tmp_path / "ds")


# ── /classify/train ─────────────────────────────────────────────────────────
def test_classify_train_rejects_empty_samples():
    resp = client.post("/classify/train", json={"samples": [], "base_model": "yolov8n-cls"})
    assert resp.status_code == 400


def test_classify_train_size_guard_blocks_large(tmp_path):
    f = tmp_path / "a.jpg"
    f.write_bytes(b"x")
    sample = {"image_path": str(f), "label": "aberto"}
    sample2 = {"image_path": str(f), "label": "fechado"}
    resp = client.post(
        "/classify/train",
        json={"samples": [sample, sample2], "base_model": "yolov8x-cls"},
    )
    assert resp.status_code == 400


def test_classify_train_saves_to_named_model(tmp_path, monkeypatch):
    models = tmp_path / "models"
    monkeypatch.setattr(main, "MODEL_DIR", models)
    f = tmp_path / "a.jpg"
    f.write_bytes(b"x")
    samples = [
        {"image_path": str(f), "label": "aberto"},
        {"image_path": str(f), "label": "fechado"},
    ]
    resp = client.post(
        "/classify/train",
        json={"samples": samples, "base_model": "yolov8n-cls", "model": "custom-cls-9", "epochs": 1},
    )
    assert resp.status_code == 200
    job = resp.json()["job_id"]
    status = {}
    for _ in range(60):
        status = client.get(f"/finetune/status/{job}").json()
        if status["status"] in ("done", "error"):
            break
        time.sleep(0.05)
    assert status.get("status") == "done", status
    assert (models / "custom-cls-9.pt").exists(), "modelo deveria ser salvo no nome pedido"


def test_classify_train_disables_horizontal_flip(tmp_path, monkeypatch):
    # Classes direcionais (entrando/saindo) seriam corrompidas por espelhamento
    # horizontal — o treino de classify deve passar fliplr=0.0.
    monkeypatch.setattr(main, "MODEL_DIR", tmp_path / "models")
    FakeYOLO.last_train_kwargs = None
    f = tmp_path / "a.jpg"
    f.write_bytes(b"x")
    samples = [
        {"image_path": str(f), "label": "com-pessoa-entrando"},
        {"image_path": str(f), "label": "com-pessoa-saindo"},
    ]
    resp = client.post(
        "/classify/train",
        json={"samples": samples, "base_model": "yolov8n-cls", "epochs": 1},
    )
    assert resp.status_code == 200
    job = resp.json()["job_id"]
    for _ in range(60):
        status = client.get(f"/finetune/status/{job}").json()
        if status["status"] in ("done", "error"):
            break
        time.sleep(0.05)
    assert FakeYOLO.last_train_kwargs is not None, "model.train não foi chamado"
    assert FakeYOLO.last_train_kwargs.get("fliplr") == 0.0


def test_classify_train_returns_job_id(tmp_path):
    f = tmp_path / "a.jpg"
    f.write_bytes(b"x")
    samples = [
        {"image_path": str(f), "label": "aberto"},
        {"image_path": str(f), "label": "fechado"},
    ]
    resp = client.post(
        "/classify/train",
        json={"samples": samples, "base_model": "yolov8n-cls", "epochs": 1},
    )
    assert resp.status_code == 200
    assert "job_id" in resp.json()
