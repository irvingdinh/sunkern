.PHONY: install kill run

install:
	@cd framework && go mod download
	@cd service && go mod download

kill:
	@-lsof -ti :19110 | xargs kill -9 2>/dev/null; true

run: kill
	@cd service && go run .
