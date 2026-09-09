From python:3.11-slim AS flag-service

WORKDIR /app

COPY flag-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY flag-service/* ./
CMD ["python", "app.py"]
