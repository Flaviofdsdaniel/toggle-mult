FROM python:3.11-slim AS targeting-service

WORKDIR /app
COPY targeting-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY targeting-service/ .
CMD ["python", "app.py"]
