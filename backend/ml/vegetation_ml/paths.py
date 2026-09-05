"""Пути по умолчанию.

Команды пакета запускаются из `backend/ml/`, а данные и артефакты лежат в корне
репозитория. Пути считаются от корня, чтобы значения по умолчанию работали
независимо от текущего каталога.
"""

from __future__ import annotations

from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]

TRAIN_CSV = str(REPO_ROOT / "data" / "train_dataset.csv")
PRIVATE_CSV = str(REPO_ROOT / "data" / "private_features.csv")
TEST_CSV = str(REPO_ROOT / "data" / "test_features.csv")
SUBMISSION_CSV = str(REPO_ROOT / "submission.csv")
MODEL_ARTIFACT = str(REPO_ROOT / "artifacts" / "restoration.pkl")
REPORTS_DIR = REPO_ROOT / "reports"
