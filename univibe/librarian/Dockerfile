# UPGRADE TO PYTHON 3.11 (Required for modern yt-dlp)
FROM python:3.11-slim

WORKDIR /app

# Install system dependencies (gcc is needed for some python packages)
RUN apt-get update && apt-get install -y gcc

# Copy requirements
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY . .

# Run unbuffered
CMD ["python", "-u", "worker.py"]