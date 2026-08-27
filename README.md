# Accountant CRM

A multi-tenant accounting and CRM platform with AI-powered document processing.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CDN (Alibaba)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Application Load Balancer                            │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                               │
                    ▼                               ▼
          ┌─────────────────┐             ┌─────────────────┐
          │   Go Backend    │◄───mTLS────►│  Python AI Svc  │
          │   (Gin/REST)    │             │    (FastAPI)    │
          └─────────────────┘             └─────────────────┘
                    │                               │
        ┌───────────┼───────────┐       ┌───────────┼───────────┐
        ▼           ▼           ▼       ▼           ▼           ▼
   ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
   │  Neon   │ │  Redis  │ │   MNS   │ │  Atlas  │ │   OSS   │
   │ Postgres│ │(Apsara) │ │ Topics  │ │ MongoDB │ │ Buckets │
   └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js 14, TypeScript, Tailwind CSS |
| Backend API | Go 1.22, Gin Framework |
| AI Service | Python 3.11, FastAPI |
| Database | Neon PostgreSQL (primary), MongoDB Atlas (documents) |
| Cache | Alibaba ApsaraDB Redis |
| Storage | Alibaba Cloud OSS |
| Queue | Alibaba Cloud MNS |
| Infrastructure | Terraform, ECS, ACR |
| CI/CD | GitHub Actions |

## Prerequisites

- Docker & Docker Compose v2.x
- Go 1.22+
- Node.js 20+
- Python 3.11+
- Terraform 1.5+
- Alibaba Cloud CLI (aliyun)

## Quick Start

### 1. Clone and Configure

```bash
# Clone repository
git clone https://github.com/your-org/accountant-crm.git
cd accountant-crm

# Copy environment template
cp .env.example .env

# Edit .env with your credentials
nano .env
```

### 2. Start Local Development Stack

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Check service status
docker-compose ps
```

### 3. Verify Services

```bash
# Go Backend
curl http://localhost:8080/health
# Expected: {"status":"ok"}

curl http://localhost:8080/ready
# Expected: {"db":"ok","redis":"ok"}

# Python AI Service
curl http://localhost:8000/health
# Expected: {"status":"ok","mongodb":"ok","oss":"ok"}

# Frontend
open http://localhost:3000
```

## Development

### Go Backend

```bash
cd go-backend

# Run locally (without Docker)
go run cmd/server/main.go

# Run tests
go test ./...

# Build binary
go build -o bin/server cmd/server/main.go
```

### Python AI Service

```bash
cd python-ai

# Create virtual environment
python -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Run locally
uvicorn main:app --reload --port 8000

# Run tests
pytest
```

### Frontend

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build

# Export static files
npm run export
```

## Database Migrations

```bash
# Apply migrations
docker-compose exec postgres psql -U accountant -d accountant_crm \
  -f /docker-entrypoint-initdb.d/000001_init_schema.up.sql

# Rollback migrations
docker-compose exec postgres psql -U accountant -d accountant_crm \
  -f /docker-entrypoint-initdb.d/000001_init_schema.down.sql
```

## Infrastructure

### Terraform

```bash
cd terraform

# Initialize
terraform init

# Plan staging deployment
terraform plan -var-file=staging.tfvars

# Apply staging deployment
terraform apply -var-file=staging.tfvars

# Plan production deployment
terraform plan -var-file=prod.tfvars
```

### Manual Deployment

```bash
# Deploy frontend to OSS
./scripts/deploy-frontend.sh staging

# Production deployment (requires approval)
./scripts/deploy-frontend.sh production
```

## API Endpoints

### Go Backend (Port 8080)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe (DB + Redis) |
| POST | `/api/v1/auth/login` | User authentication |
| POST | `/api/v1/auth/refresh` | Token refresh |
| GET | `/api/v1/tenants` | List tenants |

### Python AI Service (Port 8000)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Service health check |
| POST | `/api/v1/ocr/extract` | Extract text from document |
| POST | `/api/v1/classify` | Classify document type |
| POST | `/api/v1/chat` | AI chat completion |

## Secrets Setup

### Local Development

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your actual credentials (file is gitignored)

### Infrastructure (Terraform)

1. Copy the example tfvars:
```bash
cp terraform/terraform.tfvars.example terraform/terraform.tfvars
```

2. Edit `terraform/terraform.tfvars` with your credentials (file is gitignored)

### Security Notes

| File | Purpose | Git Status |
|------|---------|------------|
| `.env` | Local dev secrets | Gitignored |
| `terraform/terraform.tfvars` | Infrastructure secrets | Gitignored |
| `terraform/tfplan` | Terraform plan output | Gitignored |

> **Note**: Secrets are passed to ECI containers as environment variables at deploy time. No external secrets manager required for staging.

---

## Environment Variables

See [.env.example](.env.example) for complete list.

### Required Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | Neon PostgreSQL connection string |
| `MONGODB_URI` | MongoDB Atlas connection string |
| `REDIS_HOST` | ApsaraDB Redis host |
| `ALIBABA_ACCESS_KEY_ID` | Alibaba Cloud credentials |
| `ALIBABA_ACCESS_KEY_SECRET` | Alibaba Cloud credentials |
| `JWT_SECRET_KEY` | 256-bit secret for JWT signing |

## Feature Flags

AI features can be toggled via Redis keys:

```bash
# Enable/disable OCR
redis-cli SET ai:ocr:enabled "true"

# Enable/disable Chat
redis-cli SET ai:chat:enabled "true"

# Enable/disable Form Extraction
redis-cli SET ai:forms:enabled "true"
```

## Project Structure

```
.
├── .github/workflows/     # CI/CD pipelines
├── go-backend/            # Go API service
│   ├── cmd/server/        # Application entrypoint
│   └── internal/          # Internal packages
├── python-ai/             # Python AI service
│   └── internal/          # Internal modules
├── frontend/              # Next.js application
│   ├── app/               # App router pages
│   └── components/        # React components
├── terraform/             # Infrastructure as code
├── migrations/            # Database migrations
├── scripts/               # Utility scripts
└── docker-compose.yml     # Local development
```

## Monitoring

- **Health Checks**: `/health` endpoints on all services
- **Readiness**: `/ready` validates external dependencies
- **Logs**: Structured JSON logging to stdout
- **Metrics**: OpenTelemetry (planned)

## Security

- mTLS between Go and Python services
- JWT authentication with refresh token rotation
- Row-level security for multi-tenancy
- Secrets in gitignored files (`terraform.tfvars`, `.env`)
- ECI containers receive secrets as env vars at deploy
- No PII in logs

See [Final_md/SECURITY.md](Final_md/SECURITY.md) for full security documentation.

## Contributing

1. Create feature branch from `main`
2. Make changes with tests
3. Submit PR with description
4. Wait for CI and code review
5. Merge after approval

## License

Proprietary - All rights reserved
