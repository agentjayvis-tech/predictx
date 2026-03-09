"""
Resolution Service — FastAPI application entry point.

Run:
  uvicorn resolution_service.main:app --host 0.0.0.0 --port 8001

Celery worker:
  celery -A resolution_service.tasks.celery_app worker --loglevel=info

Celery beat scheduler:
  celery -A resolution_service.tasks.celery_app beat --loglevel=info
"""

import logging

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .api import appeals_router, community_router, insurance_router, judges_router, router
from .database import init_db
from .events import stop_producer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s — %(message)s",
)

app = FastAPI(
    title="PredictX — Resolution Service",
    description=(
        "Handles market resolution via automated APIs (sports/weather), "
        "manual judge flow, community dispute reporting, appeals, "
        "and the insurance fund."
    ),
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# Register all routers
app.include_router(router)
app.include_router(judges_router)
app.include_router(community_router)
app.include_router(appeals_router)
app.include_router(insurance_router)


@app.on_event("startup")
async def startup():
    await init_db()


@app.on_event("shutdown")
async def shutdown():
    await stop_producer()


@app.get("/health", tags=["health"])
async def health():
    return {"status": "ok", "service": "resolution-service"}
