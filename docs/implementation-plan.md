# Feedway MVP — plan implementacji

Ten plik jest jedynym backlogiem projektu. `spec.md` definiuje kontrakt MVP, a
`docs/future-ideas.md` jest wyłącznie niepriorytetyzowanym parking lotem.

## Zasady realizacji

Każda paczka przechodzi cykl: implementacja → `just test` w Docker Compose →
review użytkownika → poprawki → ponowny test → jawna akceptacja → jeden lokalny
commit. Push, PR, tag, release i publikacja obrazu wymagają osobnego polecenia.

Checkbox zamyka się po akceptacji, końcowym teście i commicie. Diff obejmuje
tylko bieżącą paczkę, `.agents/` nie jest modyfikowane przez formatowanie, a kod
nie zawiera funkcji ani abstrakcji wykraczających poza MVP.

Projekt stosuje KISS i convention over configuration w duchu Rails/DHH. Nowa
opcja, abstrakcja, fallback albo extension point wymaga konkretnej aktualnej
potrzeby. Konwencje są preferowane nawet wtedy, gdy późniejsza potrzeba może
wymagać jawnej zmiany kontraktu.

## Stały kontrakt

- jeden hardcoded feed `Feedway` pod `/feed.json`;
- tylko `POST /api/v1/entries`, `GET`/`HEAD /feed.json` i endpointy operacyjne;
- wymagane `external_id` jest primary key i publicznym JSON Feed `item.id`;
- wymagane `content_html`, opcjonalny `title`;
- bez UUID, hasha, deduplikacji treści, cursorów i management API;
- `net/http`, bez Chi, Huma, OpenAPI i Swagger UI;
- konfiguracja: `DATABASE_URL`, `API_TOKEN` i zachowany `MIGRATIONS_MODE`;
- pełna operacyjność, skille, CI i release pozostają w MVP.

## Zależności

Przed dodaniem modułu, narzędzia, obrazu lub Action należy sprawdzić najnowszą
stabilną wersję w oficjalnej dokumentacji i upstreamie.

- Go 1.26.x, PostgreSQL 18.x, pgx/v5, Bluemonday i Tern v2;
- bez prerelease bez jawnej zgody;
- obrazy przypięte digestem, Actions pełnym SHA z komentarzem wersji;
- narzędzia Go przez `go get -tool`;
- dodatkowa zależność runtime wymaga uzasadnienia w review.

Renovate: `gomod`, `dockerfile`, `docker-compose` i `github-actions`, z
`gomodTidy`, dependency dashboard oraz pinowaniem digestów i SHA. PostgreSQL jest
ograniczony do 18.x. Non-major Go/Docker/tools, digesty i wszystkie aktualizacje
Actions mają automerge po zielonym CI. Pozostałe major wymagają review.

## Repo-lokalne skille Go

P1 instaluje z `samber/cc-skills-golang`, przez `npx skills add ... --copy`, dla
wszystkich wspieranych agentów, dokładnie:

- `golang-project-layout`
- `golang-code-style`
- `golang-naming`
- `golang-modernize`
- `golang-context`
- `golang-error-handling`
- `golang-testing`
- `golang-database`
- `golang-concurrency`
- `golang-security`
- `golang-dependency-management`
- `golang-lint`
- `golang-continuous-integration`

Odbiór: `npx skills list --json` pokazuje dokładnie ten zestaw, a
`.agents/skills` i `skills-lock.json` są zgodne. Aktualizacje wykonuje się przez
`npx skills update -p -y` i poddaje osobnemu review. `.agents/` jest wyłączone z
formatterów i pre-commit.

Nie instalujemy `golang-swagger` ani skilli dotyczących observability, Cobra,
DI, Viper, ORM, GraphQL, gRPC, Testify i optymalizacji. Nie kopiujemy `.codex` z
Sourcetap.

## Bootstrap

- [x] **P0 — specyfikacja i kontrakt pracy**
  - dodać plan, `AGENTS.md` i niebacklogowy `docs/future-ideas.md`;
  - zapisać workflow review-before-commit, KISS i convention over configuration;
  - ograniczyć produkt do jednego hardcoded feeda i wymaganego `external_id`;
  - pierwszy commit obejmuje zastany `spec.md`.

- [x] **P1 — skille**
  - zainstalować dokładny zestaw przez `npx skills add ... --copy`;
  - sprawdzić katalogi, lock i `npx skills list --json`;
  - wykluczyć `.agents/` z narzędzi modyfikujących pliki.

- [x] **P2 — dockerowe testy**
  - moduł Go, `Justfile`, `.dockerignore`, testowy Dockerfile i
    `compose.test.yaml`;
  - najnowsze stabilne Go 1.26.x i PostgreSQL 18.x z digestami;
  - `just test` uruchamia wszystkie testy z `-race` i zawsze sprząta Compose;
  - `just test-unit`, `just test-integration` i `just ci`.

- [x] **P3 — Renovate**
  - dodać i zwalidować `renovate.json5`;
  - wdrożyć opisaną politykę;
  - `renovate-config-validator --strict`.

## Aplikacja

- [x] **P4 — konfiguracja i CLI**
  - tylko `DATABASE_URL`, `API_TOKEN` i `MIGRATIONS_MODE`;
  - pozostałe wartości jako hardcoded conventions;
  - `feedway serve` i `feedway migrate`; brak komendy zwraca usage i kod 2;
  - testy konfiguracji i CLI.

- [ ] **P5 — HTTP i reguły transportu**
  - standardowe `net/http`, `/healthz`, proste błędy JSON i limit body;
  - Bearer auth wyłącznie dla POST;
  - request ID, strukturalne logi i stałoczasowe porównanie tokenów;
  - graceful shutdown.

- [ ] **P6 — PostgreSQL, migracje i readiness**
  - `pgxpool`, Tern i embedded migration jednej tabeli `entries`;
  - `external_id text PRIMARY KEY`, bez UUID i hashy;
  - advisory lock, `migrate`, dokładna wersja schematu w trybie `off`;
  - `/readyz`, timeouty, lifecycle poola i testy awarii/shutdownu.

- [ ] **P7 — sanitizacja HTML**
  - normalizacja CR/LF i whitespace;
  - Bluemonday, dozwolone elementy/protokoły i link hardening;
  - testy bezpieczeństwa, limitów i pustego wyniku.

- [ ] **P8 — atomowy upsert**
  - `POST /api/v1/entries` z wymaganymi `external_id` i `content_html`;
  - opcjonalny `title`;
  - pojedynczy `INSERT ... ON CONFLICT DO UPDATE`;
  - wyniki `created`, `updated` i `unchanged`;
  - zachowanie `created_at` i testy równoległych publikacji.

## Publikacja i operacyjność

- [ ] **P9 — JSON Feed 1.1**
  - hardcoded `title: Feedway`, brak opisu, `home_page_url` i `feed_url`;
  - `external_id` jako publiczne `item.id`;
  - wyłącznie `content_html`, opcjonalny tytuł i daty z bazy;
  - hardcoded limit 100 najnowszych wpisów.

- [ ] **P10 — publiczny endpoint i cache**
  - tylko `GET` i `HEAD /feed.json`;
  - `application/feed+json; charset=utf-8`;
  - ETag, Last-Modified, Cache-Control i conditional requests;
  - 422 po przekroczeniu hardcoded 1 MiB;
  - testy pustego feeda, 304, HEAD i granic bajtów.

- [ ] **P11 — retencja**
  - hardcoded 30 dni po `created_at`, cykl 24h i batch 1000;
  - advisory lock, obsługa wielu replik i bezpieczny shutdown.

- [ ] **P12 — obraz i Compose**
  - statyczny distroless non-root, debug target i PostgreSQL 18;
  - read-only filesystem, brak shella/capabilities, amd64/arm64;
  - bez artefaktów Kubernetes.

- [ ] **P13 — CI**
  - `just ci`: format, race, PostgreSQL 18, vet, golangci-lint, govulncheck,
    `go mod verify` i build obrazu;
  - aktualne Actions przypięte do SHA;
  - kontrola czystości `go.mod` i `go.sum`.

- [ ] **P14 — release i dokumentacja**
  - pełne testy, multi-arch build, wersjonowane tagi, SBOM i skan obrazu;
  - README: Compose, curl, n8n, backup, migracje, upgrade i troubleshooting;
  - smoke z aktualnym Miniflux: `/feed.json`, upsert, ETag i 304;
  - po odbiorze przygotować `v0.1.0`; Cosign opcjonalny po konfiguracji registry.
