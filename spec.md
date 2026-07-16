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
- konfiguracja tylko dla wartości zależnych od środowiska;
- żadnych abstrakcji, fallbacków ani extension points bez aktualnej potrzeby;
- nowe możliwości dopiero po potwierdzeniu rzeczywistego zapotrzebowania.

Nie oznacza to kopiowania architektury Rails do Go. Kod ma być bezpośredni,
idiomatyczny dla Go i oparty głównie na bibliotece standardowej.

## 2. Zakres MVP

- `POST /api/v1/entries` z automatyczną deduplikacją finalnej treści;
- `GET` i `HEAD /feed.json` jako JSON Feed 1.1;
- hardcoded limit 100 najnowszych wpisów;
- bezpieczny, sanitizowany `content_html`;
- PostgreSQL 18.x i automatycznie stosowany embedded schema;
- Bearer Token dla publikacji;
- liveness, proste readiness i graceful shutdown;
- strukturalne logi JSON;
- ETag, Cache-Control i conditional requests;
- hardcoded retencja 30 dni;
- Docker Compose, distroless non-root i amd64/arm64;
- testy jednostkowe, integracyjne i race;
- repo-lokalne skille Go, Renovate i CI.

Pomysły świadomie odłożone poza MVP są zapisane w `docs/future-ideas.md`. Ten
plik nie jest backlogiem.

## 3. Kontrakt wpisu

Request publikacji:

```json
{
  "title": "Daily report",
  "content_html": "<p>New releases are available.</p>"
}
```

Pola:

- `content_html` — wymagany fragment HTML, maksymalnie 256 KiB przed i po
  sanitizacji;
- `title` — opcjonalny zwykły tekst, maksymalnie 1000 znaków.

Klient nie podaje identyfikatora. Po normalizacji i sanitizacji aplikacja
oblicza wersjonowany SHA-256 z finalnego `title` i `content_html`. Wynik w
formacie `sha256-v1:<hex>` jest primary key w PostgreSQL i publicznym JSON Feed
`item.id`.

Identyczna finalna treść ma zawsze ten sam identyfikator i nie tworzy kolejnego
wpisu. Zmieniony tytuł albo HTML tworzy nowy, niezmienny wpis. Nie generujemy
UUID i nie aktualizujemy istniejących wpisów.

Możliwe wyniki:

- `created` — HTTP 201;
- `deduplicated` — HTTP 200.

## 4. HTTP API

### 4.1. Publikacja

```http
POST /api/v1/entries
Authorization: Bearer <API_TOKEN>
Content-Type: application/json
```

To jedyny endpoint zapisu. Nie ma API listowania, pobierania, edycji ani
usuwania wpisów.

Request body ma hardcoded limit 1 MiB. Odpowiedź zawiera tylko wynik i
wygenerowany identyfikator:

```json
{
  "result": "created",
  "id": "sha256-v1:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
```

Ponowienie tej samej finalnej treści zwraca `result: "deduplicated"` oraz ten
sam identyfikator.

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
- `GET /readyz` — zakończony start, ping PostgreSQL z krótkim timeoutem i stan
  shutdownu;

Serwer używa standardowego `net/http`. MVP nie zawiera Chi, Huma, OpenAPI ani
Swagger UI.

### 4.4. Błędy aplikacyjne

Błędy aplikacyjne mają prostą, stabilną odpowiedź JSON:

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

Nieznane ścieżki i niedozwolone metody pozostają standardowymi odpowiedziami
404 i 405 z `net/http`.

Błąd nie ujawnia SQL, nazw constraintów, konfiguracji, sekretów ani stack trace.

## 5. Normalizacja i sanitizacja

Kolejność publikacji:

1. walidacja JSON i wymaganych pól;
2. zamiana CRLF i CR na LF;
3. obcięcie zewnętrznych białych znaków;
4. zamiana pustego `title` na `NULL`;
5. sanitizacja HTML;
6. sprawdzenie, czy `content_html` pozostał niepusty;
7. obliczenie wersjonowanego SHA-256 finalnego tytułu i HTML;
8. atomowy insert z deduplikacją w PostgreSQL.

Nie zwijamy białych znaków wewnątrz treści i nie kanonikalizujemy DOM.

Sanitizacja używa bez modyfikacji konserwatywnej polityki `bluemonday.UGCPolicy`.
Nie utrzymujemy własnej listy elementów, atrybutów ani protokołów. Jeżeli
rzeczywisty wpis wymaga dodatkowego bezpiecznego elementu, politykę rozszerzymy
w osobnej, testowanej zmianie. Jeżeli po sanitizacji nie pozostaje treść, API
zwraca 422. Oryginalny HTML nie jest przechowywany.

## 6. PostgreSQL

Jedyna tabela domenowa:

```sql
CREATE TABLE entries (
    id           text PRIMARY KEY,
    title        text,
    content_html text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT entries_id_valid CHECK (
        id ~ '^sha256-v1:[0-9a-f]{64}$'
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
    ON entries(created_at DESC, id DESC);
```

Publikacja używa pojedynczego `INSERT ... ON CONFLICT (id) DO NOTHING`. Nie
wykonuje wcześniejszego SELECT-a.

Schemat jednej tabeli jest osadzony przez `embed` i stosowany automatycznie
przed rozpoczęciem nasłuchiwania. MVP nie ma osobnej komendy migracji, trybów
migracji ani zależności od frameworka migracyjnego.

## 7. JSON Feed i cache

Feed publikuje 100 najnowszych wpisów według
`created_at DESC, id DESC`.

Item:

```json
{
  "id": "sha256-v1:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "title": "Daily report",
  "content_html": "<p>New releases are available.</p>",
  "date_published": "2026-07-15T08:00:00Z"
}
```

`title` jest pomijany, gdy go nie ma. `date_published` odpowiada `created_at`.

Finalny, nieskompresowany JSON ma hardcoded limit 1 MiB. Przekroczenie zwraca
422; aplikacja nie publikuje cicho obciętej reprezentacji.

ETag to SHA-256 finalnego JSON-u. Trafienie `If-None-Match` zwraca 304 bez body.
HEAD zwraca nagłówki GET bez body. MVP nie implementuje `Last-Modified` ani
`If-Modified-Since`.

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

Wszystkie pozostałe wartości są konwencjami w kodzie:

- listen address `:8080`;
- limit requestu i feeda 1 MiB;
- limit treści 256 KiB;
- 100 wpisów w feedzie;
- retencja 30 dni i cleanup co 24 godziny;
- standardowe timeouty HTTP, DB i shutdown;
- log level `info`, format JSON.

Brak `DATABASE_URL` albo `API_TOKEN` uniemożliwia start.

## 9. Retencja

Retencja usuwa jednym zapytaniem wpisy, których `created_at` jest starsze niż
30 dni. Aktualizacja wpisu nie przedłuża jego życia. Cleanup wykonuje się po
starcie i co 24 godziny. Jest idempotentny, respektuje shutdown i nie używa
batchy, advisory locków ani konfiguracji.

## 10. Logi i shutdown

Logowanie używa `log/slog` w formacie JSON. Logi obejmują metodę, route, status,
czas, wygenerowane `id` i wynik publikacji. Nie obejmują Authorization, API_TOKEN,
DATABASE_URL, request body ani pełnej treści wpisu. Udane probe'y `GET /healthz`
i `GET /readyz` nie są logowane.

Po SIGTERM lub SIGINT aplikacja kolejno:

1. wyłącza readiness;
2. zatrzymuje przyjmowanie nowych requestów;
3. kończy aktywne requesty;
4. anuluje cleanup retencji;
5. zamyka pool PostgreSQL;
6. kończy proces w hardcoded timeoutcie 15 sekund.

## 11. Uruchomienie

Binarka nie ma komend ani flag. Jej uruchomienie automatycznie przygotowuje
schemat i startuje serwer.

## 12. Dostarczenie

- Go 1.26.x i PostgreSQL 18.x, najnowszy stabilny patch w ramach major;
- runtime dependencies: pgx/v5 i Bluemonday;
- narzędzia Go zapisane przez `go get -tool`;
- obrazy bazowe przypięte digestem;
- statyczny distroless Debian 13 non-root bez shella;
- Compose: read-only filesystem, brak capabilities i eskalacji uprawnień;
- obrazy `linux/amd64` i `linux/arm64`;
- brak Docker HEALTHCHECK; Compose używa endpointów HTTP;
- brak artefaktów Kubernetes.

## 13. Testy i CI

`just test` uruchamia testy z `-race` w Docker Compose i zawsze sprząta zasoby.
Integracja używa efemerycznego PostgreSQL 18.x. Dostępne są też
`just test-unit`, `just test-integration` i `just ci`.

Testy obejmują:

- konfigurację i transport HTTP;
- normalizację i sanitizację HTML;
- deterministyczny hash, `created` i `deduplicated`;
- równoległe publikacje tej samej treści;
- automatyczne przygotowanie schematu i readiness;
- retencję;
- pusty feed, wpis bez tytułu, limit 100 i limit bajtów;
- ETag, 304 i HEAD;
- auth, logi i graceful shutdown.

`just ci` wykonuje format check, race, integrację z PostgreSQL 18, vet,
golangci-lint, govulncheck, `go mod verify`, kontrolę czystości modułu i build
obrazu.

Renovate obsługuje `gomod`, `dockerfile`, `docker-compose` i `github-actions`,
wykonuje `gomodTidy`, przypina digesty oraz pełne SHA Actions. PostgreSQL jest
ograniczony do 18.x. Non-major Go/Docker/tools, digesty i wszystkie aktualizacje
Actions mogą być automatycznie scalane po zielonym CI; pozostałe major wymagają
ręcznego review.

Pierwsze wydanie powstaje po odbiorze MVP. Rozbudowana automatyzacja release,
SBOM, skanowanie obrazu i podpisywanie nie należą do MVP.

## 14. Definition of Done

MVP jest gotowe, gdy użytkownik może:

1. uruchomić Feedway i PostgreSQL przez Docker Compose;
2. opublikować sanitizowany HTML przez `POST /api/v1/entries`;
3. dodać publiczny adres `/feed.json` do aktualnego Miniflux;
4. ponowić publikację tej samej treści bez utworzenia duplikatu;
5. otrzymać stabilny ETag i 304;
6. potwierdzić health, readiness, retencję i graceful shutdown;
7. wykonać backup i upgrade według README.

Proces działa jako non-root, bez shella, z read-only filesystem i przechodzi
pełne `just ci`.

## 15. Plan implementacji

Jedynym backlogiem jest `docs/implementation-plan.md`.
