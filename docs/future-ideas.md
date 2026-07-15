# Feedway — future ideas

Ten dokument jest niepriorytetyzowanym parking lotem pomysłów świadomie
wyłączonych z MVP. Nie jest backlogiem ani obietnicą implementacji.

Pomysł trafia do `docs/implementation-plan.md` dopiero po pojawieniu się
konkretnej potrzeby, świadomej decyzji o zmianie kontraktu i określeniu
minimalnego kryterium odbioru.

## Więcej feedów

- tabela `feeds` i identyfikatory feedów;
- tworzenie, listowanie, aktualizacja i usuwanie feedów;
- konfigurowalne tytuły, opisy i publiczne URL-e;
- prywatne feedy i tokeny per feed.

## Zarządzanie wpisami

- lista wpisów w API;
- pobieranie i usuwanie pojedynczego wpisu;
- cursor pagination;
- osobny PATCH wpisu;
- historia zmian lub soft delete.

## Alternatywna tożsamość i treść

- opcjonalne `external_id`;
- automatyczna deduplikacja treści i wersjonowany hash;
- generowane UUID wpisów;
- osobne `content_text`;
- top-level `url` wpisu;
- podawane przez klienta `published_at`;
- autorzy, tagi, załączniki i ikony.

## Powierzchnia HTTP i publikacja

- Huma, Chi, OpenAPI i Swagger UI, jeśli liczba endpointów uzasadni framework;
- Problem Details dla rozbudowanego API;
- landing page, discovery i `home_page_url`;
- opcjonalne `feed_url`;
- RSS, Atom, WebSub i paginacja publicznego feeda;
- kompresja HTTP wewnątrz aplikacji.

## Operacyjność na późniejsze potrzeby

- metryki Prometheus;
- webhooki wychodzące, kolejki i Redis;
- pełnotekstowe wyszukiwanie;
- proxy obrazów;
- manifesty Kubernetes;
- wiele tokenów, użytkownicy, role i uprawnienia;
- konfigurowanie obecnych hardcoded konwencji, ale tylko gdy realne wdrożenie
  wymaga innej wartości.
