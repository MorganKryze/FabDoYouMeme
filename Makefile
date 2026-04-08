.PHONY: dev dev-down preprod preprod-down prod prod-down

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
