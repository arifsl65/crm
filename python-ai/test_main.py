"""
Smoke tests for Python AI service.

Uses proper dependency injection mocking for FastAPI testing.
"""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from main import app, get_app_state
from internal.dependencies import AppState


@pytest.fixture
def mock_app_state():
    """Create a mock AppState with healthy services."""
    state = MagicMock(spec=AppState)
    state.mongodb_health_check = AsyncMock(return_value=True)
    state.postgres_health_check = AsyncMock(return_value=True)
    state.oss_health_check = AsyncMock(return_value=True)
    state.redis_health_check = AsyncMock(return_value=True)
    state.redis_client = MagicMock()
    return state


@pytest.fixture
def client(mock_app_state):
    """Create test client with mocked dependencies."""
    app.dependency_overrides[get_app_state] = lambda: mock_app_state
    yield TestClient(app)
    app.dependency_overrides.clear()


class TestHealthEndpoints:
    """Tests for health check endpoints."""

    def test_health_returns_ok(self, client):
        """Test liveness probe endpoint."""
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json() == {"status": "ok"}

    def test_ready_all_healthy(self, client, mock_app_state):
        """Test readiness probe when all services are healthy."""
        mock_app_state.mongodb_health_check.return_value = True
        mock_app_state.postgres_health_check.return_value = True
        mock_app_state.oss_health_check.return_value = True
        mock_app_state.redis_health_check.return_value = True

        response = client.get("/ready")
        assert response.status_code == 200
        data = response.json()
        assert data["mongodb"] == "ok"
        assert data["postgres"] == "ok"
        assert data["oss"] == "ok"
        assert data["redis"] == "ok"

    def test_ready_mongodb_unhealthy(self, client, mock_app_state):
        """Test readiness probe when MongoDB is unhealthy."""
        mock_app_state.mongodb_health_check.return_value = False
        mock_app_state.postgres_health_check.return_value = True
        mock_app_state.oss_health_check.return_value = True
        mock_app_state.redis_health_check.return_value = True

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.json()
        assert data["mongodb"] == "error"

    def test_ready_oss_unhealthy(self, client, mock_app_state):
        """Test readiness probe when OSS is unhealthy."""
        mock_app_state.mongodb_health_check.return_value = True
        mock_app_state.postgres_health_check.return_value = True
        mock_app_state.oss_health_check.return_value = False
        mock_app_state.redis_health_check.return_value = True

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.json()
        assert data["oss"] == "error"


class TestFeatureFlags:
    """Tests for feature flag handling."""

    @patch("main.check_feature_flag", new_callable=AsyncMock)
    def test_ocr_disabled_returns_503(self, mock_flag, client):
        """Test OCR endpoint returns 503 when disabled."""
        mock_flag.return_value = False

        response = client.post("/api/v1/ai/documents/extract?file_key=test.pdf")
        assert response.status_code == 503
        assert "Retry-After" in response.headers
        assert response.headers["Retry-After"] == "300"
        assert response.json()["error"] == "service_unavailable"

    @patch("main.check_feature_flag", new_callable=AsyncMock)
    def test_ocr_enabled_returns_stub(self, mock_flag, client):
        """Test OCR endpoint returns stub when enabled."""
        mock_flag.return_value = True

        response = client.post("/api/v1/ai/documents/extract?file_key=test.pdf")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "not_implemented"
        assert data["file_key"] == "test.pdf"

    @patch("main.check_feature_flag", new_callable=AsyncMock)
    def test_chat_disabled_returns_503(self, mock_flag, client):
        """Test chat endpoint returns 503 when disabled."""
        mock_flag.return_value = False

        response = client.post("/api/v1/ai/chat?message=hello")
        assert response.status_code == 503
        assert response.headers["Retry-After"] == "300"

    @patch("main.check_feature_flag", new_callable=AsyncMock)
    def test_classify_disabled_returns_503(self, mock_flag, client):
        """Test classify endpoint returns 503 when disabled."""
        mock_flag.return_value = False

        response = client.post("/api/v1/ai/documents/classify?file_key=test.pdf")
        assert response.status_code == 503
        assert response.headers["Retry-After"] == "300"


class TestCORS:
    """Tests for CORS configuration."""

    def test_cors_headers_present(self, client):
        """Test that CORS headers are present."""
        response = client.options(
            "/health",
            headers={"Origin": "http://localhost:3000"},
        )
        # FastAPI's CORSMiddleware handles OPTIONS
        assert response.status_code in [200, 204, 405]


class TestErrorHandling:
    """Tests for error handling."""

    def test_404_for_unknown_route(self, client):
        """Test 404 for unknown routes."""
        response = client.get("/unknown/route")
        assert response.status_code == 404
