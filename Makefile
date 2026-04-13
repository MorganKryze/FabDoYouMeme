.PHONY: dev dev-down preprod preprod-down prod prod-down env-check env-diff env-migrate

dev:
	docker compose -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev up --build -d

dev-down:
	docker compose -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev down

preprod:
	docker compose -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod up --build -d

preprod-down:
	docker compose -f docker/compose.base.yml -f docker/compose.preprod.yml --env-file .env.preprod down

prod:
	docker compose -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod up -d

prod-down:
	docker compose -f docker/compose.base.yml -f docker/compose.prod.yml --env-file .env.prod down

# Env var drift detection & migration. Compares each .env.{dev,preprod,prod}
# against its .env.*.example template. Run env-check after pulling upstream.
# Scope to a single deployment with ENV=dev|preprod|prod, e.g. `make env-diff ENV=prod`.
env-check:
	@./scripts/env-migrate.sh check $(ENV)

env-diff:
	@./scripts/env-migrate.sh diff $(ENV)

env-migrate:
	@./scripts/env-migrate.sh migrate $(ENV)
