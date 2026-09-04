# Образ ML-сервиса (зона кода — xsqclown, Dockerfile — B4). Сборка из корня
# репозитория: docker build -f backend/ml/Dockerfile .
# Сервис загружает версионированный артефакт модели один раз на процесс;
# обучение при сборке и старте не выполняется. Артефакт входит в образ либо
# монтируется только для чтения по версии (MODEL_ARTIFACT / MODEL_VERSION).
FROM python:3.12-slim

WORKDIR /app/ml

# Зафиксированные зависимости ставятся отдельным слоем.
COPY backend/ml/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Пакет vegetation_ml и HTTP-адаптер поставляет ML-владелец.
COPY backend/ml/ ./

# Версионированный артефакт модели: файл в образе или read-only монтирование.
ENV MODEL_ARTIFACT=/app/ml/artifacts/model
ENV MODEL_VERSION=unset

EXPOSE 8000
CMD ["python", "-m", "vegetation_ml.http"]
