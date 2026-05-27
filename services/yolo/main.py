from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from ultralytics import YOLO
import cv2
import os
from collections import defaultdict

app = FastAPI()
_models: dict[str, YOLO] = {}


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
    frame_skip = 5  # analyze every 5th frame
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
