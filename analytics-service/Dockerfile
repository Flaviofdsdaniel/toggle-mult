From python:3.11-slim
WORKDIR /app
COPY analytics-service/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY analytics-service/* .
CMD ["python", "app.py"]