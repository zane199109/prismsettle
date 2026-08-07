# PrismSettle FastAPI Query Service
# Lightweight Python service for read-only agent queries.

FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 9200

CMD ["uvicorn", "offchain.fastapi.main:app", "--host", "0.0.0.0", "--port", "9200"]