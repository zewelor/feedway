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
- automatyczny SHA-256 finalnej treści jest primary key i JSON Feed `item.id`;
- wymagane `content_html`, opcjonalny `title`, bez identyfikatora w request;
- bez UUID, aktualizacji wpisów, cursorów i management API;
- `net/http`, bez Chi, Huma, OpenAPI i Swagger UI;
- konfiguracja: tylko `DATABASE_URL` i `API_TOKEN`;
- operacyjność, skille, CI i ręczny odbiór pozostają w MVP.

## Zależności

Przed dodaniem modułu, narzędzia, obrazu lub Action należy sprawdzić najnowszą
stabilną wersję w oficjalnej dokumentacji i upstreamie.

- Go 1.26.x, PostgreSQL 18.x, pgx/v5 i Bluemonday;
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
  - ograniczyć produkt do jednego hardcoded feeda;
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

- [x] **P4 — konfiguracja i start**
  - tylko `DATABASE_URL` i `API_TOKEN`;
  - pozostałe wartości jako hardcoded conventions;
  - uruchomienie binarki startuje aplikację bez komend i flag;
  - testy konfiguracji.

- [x] **P5 — HTTP i reguły transportu**
  - standardowe `net/http`, `/healthz`, proste błędy JSON i limit body;
  - Bearer auth wyłącznie dla POST;
  - strukturalne logi i stałoczasowe porównanie tokenów;
  - graceful shutdown.

- [x] **P6 — PostgreSQL, migracje i readiness**
  - `pgxpool` i embedded schema jednej tabeli `entries`;
  - `id text PRIMARY KEY` dla SHA-256, bez UUID i `updated_at`;
  - automatyczne, idempotentne przygotowanie schematu przed startem HTTP;
  - `/readyz`, timeouty, lifecycle poola i testy awarii/shutdownu.

- [x] **P7 — normalizacja, hash i sanitizacja HTML**
  - normalizacja CR/LF i whitespace;
  - niezmodyfikowane `bluemonday.UGCPolicy`;
  - wersjonowany SHA-256 finalnego `title` i `content_html`;
  - testy bezpieczeństwa, deterministyczności, limitów i pustego wyniku.

- [x] **P8 — atomowy insert i deduplikacja**
  - `POST /api/v1/entries` z wymaganym `content_html`;
  - opcjonalny `title`;
  - pojedynczy `INSERT ... ON CONFLICT (id) DO NOTHING`;
  - wyniki `created` i `deduplicated`;
  - testy równoległych publikacji tej samej treści.

## Publikacja i operacyjność

- [x] **P9 — JSON Feed 1.1**
  - hardcoded `title: Feedway`, brak opisu, `home_page_url` i `feed_url`;
  - wygenerowany SHA-256 jako publiczne `item.id`;
  - wyłącznie `content_html`, opcjonalny tytuł i daty z bazy;
  - hardcoded limit 100 najnowszych wpisów.

- [x] **P10 — publiczny endpoint i cache**
  - tylko `GET` i `HEAD /feed.json`;
  - `application/feed+json; charset=utf-8`;
  - ETag, Cache-Control i `If-None-Match`;
  - 422 po przekroczeniu hardcoded 1 MiB;
  - testy pustego feeda, 304, HEAD i granic bajtów.

- [ ] **P11 — retencja**
  - jedno idempotentne kasowanie po `created_at` starszym niż 30 dni;
  - cleanup po starcie, co 24h i bezpieczny shutdown;
  - bez batchy, advisory locków i konfiguracji.

- [ ] **P12 — obraz i Compose**
  - statyczny distroless non-root i PostgreSQL 18;
  - read-only filesystem, brak shella/capabilities, amd64/arm64;
  - bez artefaktów Kubernetes.

- [ ] **P13 — CI**
  - `just ci`: format, race, PostgreSQL 18, vet, golangci-lint, govulncheck,
    `go mod verify` i build obrazu;
  - aktualne Actions przypięte do SHA;
  - kontrola czystości `go.mod` i `go.sum`.

- [ ] **P14 — dokumentacja i odbiór**
  - README: Compose, curl, n8n, backup, upgrade i troubleshooting;
  - smoke z aktualnym Miniflux: `/feed.json`, deduplikacja, ETag i 304;
  - po odbiorze przygotować ręczne `v0.1.0`;
  - automatyzacja release, SBOM, skan i podpisywanie są poza MVP.
