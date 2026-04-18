.PHONY: dev dev-down dev-clean preprod preprod-down preprod-clean prod prod-down prod-clean env-check env-diff env-migrate

# Each stage is scoped under its own Compose project name so Postgres
# volumes, container names, and networks are isolated per stage. Without
# `-p`, Compose derives the project from the CWD basename and all three
# stacks collide on a shared `postgres_data` volume.
DEV_PROJECT     := fabdoyoumeme-dev
PREPROD_PROJECT := fabdoyoumeme-preprod
PROD_PROJECT    := fabdoyoumeme-prod

dev:
	STAGE=dev docker compose -p $(DEV_PROJECT) -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev up --build -d

dev-down:
	STAGE=dev docker compose -p $(DEV_PROJECT) -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev down

# Tears the dev stack down AND removes its named volumes (postgres_data).
# Destructive: wipes the dev database.
dev-clean:
	STAGE=dev docker compose -p $(DEV_PROJECT) -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev down -v

preprod:
	STAGE=preprod docker compose -p $(PREPROD_PROJECT) -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod up --build -d

preprod-down:
	STAGE=preprod docker compose -p $(PREPROD_PROJECT) -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod down

preprod-clean:
	STAGE=preprod docker compose -p $(PREPROD_PROJECT) -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod down -v

prod:
	STAGE=prod docker compose -p $(PROD_PROJECT) -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod up -d

prod-down:
	STAGE=prod docker compose -p $(PROD_PROJECT) -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod down

prod-clean:
	STAGE=prod docker compose -p $(PROD_PROJECT) -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod down -v

# Env var drift detection & migration. Compares each .env.{dev,preprod,prod}
# against its .env.*.example template. Run env-check after pulling upstream.
# Scope to a single deployment with ENV=dev|preprod|prod, e.g. `make env-diff ENV=prod`.
env-check:
	@./scripts/env-migrate.sh check $(ENV)

env-diff:
	@./scripts/env-migrate.sh diff $(ENV)

env-migrate:
	@./scripts/env-migrate.sh migrate $(ENV)
