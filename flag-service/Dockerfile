From python:3.13-slim-bookworm

WORKDIR /app

COPY flag-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY flag-service/* ./
CMD ["python", "app.py"]
