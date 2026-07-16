# Feedway — specyfikacja MVP

## 1. Cel i zasady

Feedway to mały, samodzielnie hostowany serwis do publikowania wiadomości z
automatyzacji, skryptów, n8n i agentów AI jako jeden publiczny JSON Feed 1.1.

MVP ma dokładnie jeden feed wbudowany w aplikację:

- tytuł `Feedway`;
- brak opisu i konfigurowalnych metadanych;
- publiczna ścieżka `/feed.json`;
- brak identyfikatora i tabeli feeda;
- brak API zarządzania feedem;
- brak landing page, `home_page_url` i `feed_url`.

Feedway stosuje KISS oraz convention over configuration w duchu Rails/DHH:

- jedna opiniotwórcza ścieżka dla każdego przypadku użycia;
- mocne konwencje zamiast opcji;
- konfiguracja tylko dla sekretów oraz zachowanego trybu migracji;
- żadnych abstrakcji, fallbacków ani extension points bez aktualnej potrzeby;
- nowe możliwości dopiero po potwierdzeniu rzeczywistego zapotrzebowania.

Nie oznacza to kopiowania architektury Rails do Go. Kod ma być bezpośredni,
idiomatyczny dla Go i oparty głównie na bibliotece standardowej.

## 2. Zakres MVP

- `POST /api/v1/entries` z atomowym upsertem przez wymagane `external_id`;
- `GET` i `HEAD /feed.json` jako JSON Feed 1.1;
- hardcoded limit 100 najnowszych wpisów;
- bezpieczny, sanitizowany `content_html`;
- PostgreSQL 18.x i embedded migrations;
- Bearer Token dla publikacji;
- liveness, readiness i graceful shutdown;
- strukturalne logi JSON;
- ETag, Last-Modified, Cache-Control i conditional requests;
- retencja, cleanup w batchach i advisory locks;
- Docker Compose, distroless non-root, debug target i amd64/arm64;
- testy jednostkowe, integracyjne i race;
- repo-lokalne skille Go, Renovate, CI, release, SBOM i skan obrazu.

Pomysły świadomie odłożone poza MVP są zapisane w `docs/future-ideas.md`. Ten
plik nie jest backlogiem.

## 3. Kontrakt wpisu

Request publikacji:

```json
{
  "external_id": "github-monitor:2026-07-15",
  "title": "Daily report",
  "content_html": "<p>New releases are available.</p>"
}
```

Pola:

- `external_id` — wymagany, nieprzezroczysty, case-sensitive string, 1–512
  znaków po obcięciu białych znaków;
- `content_html` — wymagany fragment HTML, maksymalnie 256 KiB przed i po
  sanitizacji;
- `title` — opcjonalny zwykły tekst, maksymalnie 1000 znaków.

`external_id` jest jedyną tożsamością wpisu:

- stanowi primary key w PostgreSQL;
- jest publikowany bez zmian jako JSON Feed `item.id`;
- ponowny request aktualizuje ten sam wpis;
- nie generujemy UUID i nie obliczamy hasha deduplikacyjnego.

Pierwszy request ustawia `created_at` i `updated_at`. Upsert zachowuje
`created_at`, zastępuje `title` oraz `content_html` i zmienia `updated_at` tylko,
gdy finalna treść rzeczywiście się zmieniła.

Możliwe wyniki:

- `created` — HTTP 201;
- `updated` — HTTP 200;
- `unchanged` — HTTP 200, bez zmiany `updated_at`.

## 4. HTTP API

### 4.1. Publikacja

```http
POST /api/v1/entries
Authorization: Bearer <API_TOKEN>
Content-Type: application/json
```

To jedyny endpoint zapisu. Nie ma API listowania, pobierania, edycji ani
usuwania wpisów.

Request body ma hardcoded limit 1 MiB. Odpowiedź zawiera `result` oraz finalny
wpis po normalizacji i sanitizacji.

### 4.2. Publiczny feed

```http
GET /feed.json
HEAD /feed.json
```

Publiczny feed nie wymaga autoryzacji. Inne metody oraz wariant `/feed` nie są
obsługiwane.

```http
Content-Type: application/feed+json; charset=utf-8
Cache-Control: public, max-age=60, must-revalidate
X-Content-Type-Options: nosniff
```

Minimalna reprezentacja:

```json
{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Feedway",
  "items": []
}
```

Feed nie publikuje `home_page_url`, `feed_url`, `description`, `next_url` ani
pól o wartości `null`.

### 4.3. Endpointy operacyjne

- `GET /healthz` — liveness bez odpytywania bazy;
- `GET /readyz` — inicjalizacja, dokładna wersja schematu, ping PostgreSQL z
  krótkim timeoutem i stan shutdownu;
Serwer używa standardowego `net/http`. MVP nie zawiera Chi, Huma, OpenAPI ani
Swagger UI.

### 4.4. Błędy

Błędy mają prostą, stabilną odpowiedź JSON:

```json
{
  "error": "content_html is required"
}
```

Statusy:

- 400 — błędny JSON;
- 401 — brak albo nieprawidłowy token;
- 413 — request za duży;
- 415 — nieobsługiwany Content-Type;
- 422 — nieprawidłowe dane lub zbyt duża reprezentacja feeda;
- 500 — nieoczekiwany błąd;
- 503 — baza albo aplikacja nie jest gotowa.

Błąd nie ujawnia SQL, nazw constraintów, konfiguracji, sekretów ani stack trace.

## 5. Normalizacja i sanitizacja

Kolejność publikacji:

1. walidacja JSON i wymaganych pól;
2. zamiana CRLF i CR na LF;
3. obcięcie zewnętrznych białych znaków;
4. zamiana pustego `title` na `NULL`;
5. sanitizacja HTML;
6. sprawdzenie, czy `content_html` pozostał niepusty;
7. atomowy upsert w PostgreSQL.

Nie zwijamy białych znaków wewnątrz treści i nie kanonikalizujemy DOM.

Polityka Bluemonday dopuszcza podstawowe elementy treści, nagłówki, listy,
cytaty, `pre`, `code`, linki, obrazy, tabele, `details` i `summary`.

Dopuszczone atrybuty:

- link: `href`, `title`;
- obraz: `src`, `alt`, `title`, `width`, `height`;
- komórka tabeli: `colspan`, `rowspan`;
- details: `open`.

Linki dopuszczają absolutne `http`, `https` i `mailto`; obrazy tylko absolutne
`http` i `https`. Linki HTTP(S) otrzymują `rel="noopener noreferrer"`.

Usuwane są między innymi skrypty, style, iframe, formularze, aktywne media,
SVG, event handlery, `class`, `id`, `style`, `target`, `data:` i `javascript:`.
Jeżeli po sanitizacji nie pozostaje treść, API zwraca 422. Oryginalny HTML nie
jest przechowywany.

## 6. PostgreSQL

Jedyna tabela domenowa:

```sql
CREATE TABLE entries (
    external_id text PRIMARY KEY,
    title        text,
    content_html text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT entries_external_id_valid CHECK (
        char_length(btrim(external_id)) BETWEEN 1 AND 512
    ),
    CONSTRAINT entries_title_length CHECK (
        title IS NULL OR char_length(title) <= 1000
    ),
    CONSTRAINT entries_content_html_valid CHECK (
        nullif(btrim(content_html), '') IS NOT NULL
        AND octet_length(content_html) <= 262144
    )
);

CREATE INDEX entries_created_index
    ON entries(created_at DESC, external_id DESC);
```

Publikacja używa pojedynczego `INSERT ... ON CONFLICT ... DO UPDATE` z warunkiem
pomijającym update identycznej treści. Nie wykonuje wcześniejszego SELECT-a.

Migracje są numerowanymi plikami SQL, osadzonymi przez `embed` i wykonywanymi
przez Tern v2 pod PostgreSQL advisory lockiem.

Tryby:

- `MIGRATIONS_MODE=auto` — migracje przed rozpoczęciem nasłuchiwania;
- `MIGRATIONS_MODE=off` — brak migracji i wymaganie dokładnej wersji schematu;
- `feedway migrate` — osobne uruchomienie migracji.

## 7. JSON Feed i cache

Feed publikuje 100 najnowszych wpisów według
`created_at DESC, external_id DESC`.

Item:

```json
{
  "id": "github-monitor:2026-07-15",
  "title": "Daily report",
  "content_html": "<p>New releases are available.</p>",
  "date_published": "2026-07-15T08:00:00Z",
  "date_modified": "2026-07-15T09:00:00Z"
}
```

`title` jest pomijany, gdy go nie ma. `date_published` odpowiada `created_at`, a
`date_modified` — `updated_at`.

Finalny, nieskompresowany JSON ma hardcoded limit 1 MiB. Przekroczenie zwraca
422; aplikacja nie publikuje cicho obciętej reprezentacji.

ETag to SHA-256 finalnego JSON-u. Last-Modified to późniejsze z czasu startu
procesu i maksymalnego `updated_at` wśród opublikowanych wpisów. `If-None-Match` ma
pierwszeństwo przed `If-Modified-Since`. Trafienie zwraca 304 bez body. HEAD
zwraca nagłówki GET bez body.

Feedway nie kompresuje odpowiedzi; kompresję może zapewnić reverse proxy.

## 8. Autoryzacja i konfiguracja

`API_TOKEN` jest wymagany, ma co najmniej 32 bajty i nigdy nie jest logowany.
Serwer porównuje SHA-256 oczekiwanego i otrzymanego tokenu stałoczasowo.
Niepoprawna autoryzacja zwraca 401 i `WWW-Authenticate: Bearer`.

Konfiguracja:

| Zmienna | Domyślna | Wymagana |
| --- | ---: | ---: |
| `DATABASE_URL` | — | tak |
| `API_TOKEN` | — | tak |
| `MIGRATIONS_MODE` | `auto` | nie |

Wszystkie pozostałe wartości są konwencjami w kodzie:

- listen address `:8080`;
- limit requestu i feeda 1 MiB;
- limit treści 256 KiB;
- 100 wpisów w feedzie;
- retencja 30 dni, cleanup co 24 godziny, batch 1000;
- standardowe timeouty HTTP, DB i shutdown;
- log level `info`, format JSON.

Brak `DATABASE_URL` albo `API_TOKEN` uniemożliwia start.

## 9. Retencja

Retencja usuwa wpisy, których `created_at` jest starsze niż 30 dni. Aktualizacja
wpisu nie przedłuża jego życia.

Worker:

- startuje po readiness i wykonuje cleanup od razu;
- działa co 24 godziny;
- uzyskuje `pg_try_advisory_lock` na dedykowanym połączeniu;
- usuwa maksymalnie 1000 rekordów w batchu;
- respektuje anulowanie kontekstu pomiędzy batchami;
- pomija cykl, gdy inna replika trzyma lock;
- loguje błąd, ale nie zatrzymuje API ani nie zmienia readiness.

## 10. Logi i shutdown

Logowanie używa `log/slog` w formacie JSON. Logi obejmują metodę, route, status,
czas, `external_id` i wynik publikacji. Nie obejmują Authorization, API_TOKEN,
DATABASE_URL, request body ani pełnej treści wpisu. Udane probe'y `GET /healthz`
nie są logowane.

Po SIGTERM lub SIGINT aplikacja kolejno:

1. wyłącza readiness;
2. zatrzymuje przyjmowanie nowych requestów;
3. kończy aktywne requesty;
4. anuluje i kończy worker retencji;
5. zamyka pool PostgreSQL;
6. kończy proces w hardcoded timeoutcie 15 sekund.

## 11. CLI

```text
feedway serve
feedway migrate
```

Brak albo nieznana komenda pokazuje usage i kończy kodem 2. CLI jest
zaimplementowane bez frameworka komend.

## 12. Dostarczenie

- Go 1.26.x i PostgreSQL 18.x, najnowszy stabilny patch w ramach major;
- runtime dependencies: pgx/v5, Bluemonday i Tern v2;
- narzędzia Go zapisane przez `go get -tool`;
- obrazy bazowe przypięte digestem;
- statyczny distroless Debian 13 non-root bez shella;
- osobny debug target, niepublikowany jako `latest`;
- Compose: read-only filesystem, brak capabilities i eskalacji uprawnień;
- obrazy `linux/amd64` i `linux/arm64`;
- brak Docker HEALTHCHECK; Compose używa endpointów HTTP;
- brak artefaktów Kubernetes.

## 13. Testy, CI i release

`just test` uruchamia testy z `-race` w Docker Compose i zawsze sprząta zasoby.
Integracja używa efemerycznego PostgreSQL 18.x. Dostępne są też
`just test-unit`, `just test-integration` i `just ci`.

Testy obejmują:

- konfigurację i CLI;
- normalizację i sanitizację HTML;
- atomowy upsert, `created`, `updated` i `unchanged`;
- równoległe upserty tego samego `external_id`;
- migracje, dokładną wersję schematu i readiness;
- retencję, batche i advisory lock;
- pusty feed, wpis bez tytułu, limit 100 i limit bajtów;
- ETag, Last-Modified, 304 i HEAD;
- auth, logi i graceful shutdown.

`just ci` wykonuje format check, race, integrację z PostgreSQL 18, vet,
golangci-lint, govulncheck, `go mod verify`, kontrolę czystości modułu i build
obrazu.

Renovate obsługuje `gomod`, `dockerfile`, `docker-compose` i `github-actions`,
wykonuje `gomodTidy`, przypina digesty oraz pełne SHA Actions. PostgreSQL jest
ograniczony do 18.x. Non-major Go/Docker/tools, digesty i wszystkie aktualizacje
Actions mogą być automatycznie scalane po zielonym CI; pozostałe major wymagają
ręcznego review.

Tag release uruchamia pełne testy, multi-arch build, wersjonowane tagi, SBOM i
skan obrazu. Cosign jest opcjonalny do czasu konfiguracji registry.

## 14. Definition of Done

MVP jest gotowe, gdy użytkownik może:

1. uruchomić Feedway i PostgreSQL przez Docker Compose;
2. opublikować sanitizowany HTML przez `POST /api/v1/entries`;
3. dodać publiczny adres `/feed.json` do aktualnego Miniflux;
4. zaktualizować wpis przez `external_id` bez utworzenia duplikatu;
5. otrzymać stabilny ETag i 304;
6. potwierdzić health, readiness, retencję i graceful shutdown;
7. wykonać migrację, backup i upgrade według README.

Proces działa jako non-root, bez shella, z read-only filesystem i przechodzi
pełne `just ci`.

## 15. Plan implementacji

Jedynym backlogiem jest `docs/implementation-plan.md`.
