FROM python:3.13-slim-bookworm

WORKDIR /app
COPY targeting-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY targeting-service/ .
CMD ["opentelemetry-instrument", "python", "app.py"]
