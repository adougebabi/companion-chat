FROM python:3.13.7-slim-bookworm

WORKDIR /workspace
RUN pip install --no-cache-dir uv==0.10.12
COPY pyproject.toml uv.lock .python-version ./
COPY apps/core/pyproject.toml apps/core/pyproject.toml
RUN uv sync --locked --no-dev

COPY apps/core/alembic.ini apps/core/alembic.ini
COPY apps/core/migrations apps/core/migrations
COPY apps/core/src apps/core/src

ENV PYTHONPATH=/workspace/apps/core/src
CMD ["uv", "run", "--no-sync", "serve-api"]
