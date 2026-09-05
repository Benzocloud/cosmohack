# Образ ML-сервиса. Сборка всегда выполняется из корня репозитория:
#   docker build -f backend/ml/Dockerfile .
# Обучение при сборке и старте не выполняется: образ содержит импортированный
# артефакт и его manifest, а сервис загружает их один раз на процесс.
FROM python:3.12-slim

WORKDIR /app/ml

# Зафиксированные зависимости ставятся отдельным слоем.
COPY backend/ml/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# В runtime попадает только пакет сервиса; тесты, batch-скрипты и исходный
# manifest остаются за пределами образа.
COPY backend/ml/vegetation_ml/ ./vegetation_ml/

# Единственный production-артефакт этого выпуска. Будущие версии получают
# новый manifest и явно добавляются в отдельный выпуск образа.
COPY artifacts/restoration.pkl /app/ml/artifacts/restoration.pkl
COPY backend/ml/model-manifest.json /app/ml/artifacts/manifest.json

# Runtime не должен иметь права записи в код или модель.
RUN groupadd --system --gid 10001 vegetation \
    && useradd --system --uid 10001 --gid 10001 --no-create-home \
       --shell /usr/sbin/nologin vegetation

ENV VEGETATION_ML_MODEL=/app/ml/artifacts/restoration.pkl
ENV VEGETATION_ML_MANIFEST=/app/ml/artifacts/manifest.json

USER 10001:10001
EXPOSE 8000
ENTRYPOINT ["python", "-m", "vegetation_ml"]
