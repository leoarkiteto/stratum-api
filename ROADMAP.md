# Roadmap — Features do Síndico

Plano de implementação ordenado e lógico das features voltadas ao síndico, no
estilo do projeto: **monólito modular GOTTH** (Go + Tailwind + Templ + HTMX),
cada feature é um slice vertical em `internal/<feature>/` com
`module.go`, `handler.go`, `service.go`, `store.go`, `templates/`,
mais migração própria em `migrations/`.

## Estado atual (ponto de partida)

| Item | Status |
|---|---|
| Auth (registro/login/logout) + RBAC `syndic`/`owner`/`tenant` | ✅ feito |
| Home (landing + dashboard) | ✅ feito |
| Eleições (nomination → voting → transition → closed) | 🚧 em andamento (não commitado: migrações 0003/0004 + `internal/election/`) |
| `web.RequireRole` (gate por papel) | ❌ não existe (gates são feitos inline nos handlers) |
| Mecanismo para promover um usuário a síndico | ❌ não existe (registro só aceita `owner`/`tenant`) |
| Conceito de condomínio/unidade | ❌ não existe — **premissa: condomínio único** |

Build e testes passam hoje (`go build ./... && go test ./...` ok).

## Premissas

- **Condomínio único**: o modelo atual não tem `condos`/`units`. Todas as
  features assumem um único condomínio: "coletivo" = todos os usuários com
  papel `owner`/`tenant`. Multi-condomínio fica como iteração futura.
- **Eleições já em andamento**: a Fase 3 é *finalizar* o trabalho existente,
  não recomeçar.
- **Primeiro síndico é manual; os demais vêm das eleições**: o registro de
  usuário nunca atribui `syndic`. O primeiro síndico é definido uma única vez
  de forma manual (Fase 0, bootstrap); a partir daí, todo novo síndico é o
  vencedor de uma eleição — no fim do período de transição o vencedor vira
  `syndic` e o síndico anterior volta a `owner` (já implementado em
  `internal/election/store.go`, `SettleTransitions`).
- **Comunicações são in-app** (inbox no dashboard). E-mail/push real fica para
  depois; a estrutura de tabela já permite plugar um canal externo.

## Ordem proposta (resumo)

| # | Fase | Feature | Por que aqui | Tamanho |
|---|---|---|---|---|
| 0 | Fundação | `web.RequireRole` + promoção a síndico + nav por papel | toda feature de síndico depende disso | S |
| 1 | Prestadores de serviços | CRUD de contatos | mais simples — define o padrão de módulo com menor risco | S |
| 2 | Contas a pagar/receber | lançamentos financeiros | mesmo padrão da Fase 1, sem dependências externas | M |
| 3 | Eleições | finalizar módulo em andamento | desbloqueia o repo (trabalho não commitado) e já usa a Fase 0 | M |
| 4 | Tickets | owner/tenant criam; síndico gerencia | primeira feature voltada ao morador; separa bem os papéis | M |
| 5 | Comunicações | mensagem/aviso individual ou coletivo | vira o **backbone de notificação** reutilizado pelas reuniões | M |
| 6 | Pautas | moradores propõem; síndico aprova | conteúdo das reuniões, preparado antes delas | S |
| 7 | Reuniões | criar/agendar, com pautas + convite | capstone: consome Pautas (agenda) + Comunicações (convite) | M |

**Lógica da ordem:** fundação compartilhada primeiro (F0) → dois CRUDs simples
que estabelecem o padrão de módulo (F1/F2) → destrava o trabalho de eleições já
existente (F3) → features com interação de morador em sequência coerente:
tickets → comunicações → pautas → reuniões, onde cada fase reutiliza a anterior.

---

## Fase 0 — Fundação (sem migração)

- `web.RequireRole(roles ...model.Role) func(http.Handler) http.Handler` que
  envolve `web.RequireAuth` e responde 403/redireciona quando o papel não bate.
- Bootstrap do **primeiro** síndico: como o registro nunca atribui `syndic`,
  um alvo no `Makefile` (ex.: `make promote-syndic EMAIL=x`) executa um UPDATE
  via `psql` no docker-compose, ou um mini comando em `cmd/`. É uso único de
  setup — os próximos síndicos vêm das eleições (Fase 3), então o comando pode
  ser descartado depois ou mantido só como recovery manual.
- Nav (`internal/web/templates/base.templ`) e dashboard por papel: síndico vê
  os atalhos das fases 1–7; morador vê tickets, comunicados, pautas, reuniões.
- Aplicar `RequireRole` nas rotas de eleição que já checam papel inline
  (preparação da Fase 3).

**Pronto quando:** `make check` verde; um usuário vira síndico pelo comando e
vê a nav expandida; morador não acessa rota de síndico (403).

## Fase 1 — Prestadores de serviços (migração 0005)

- Tabela `service_providers`: `id`, `name`, `category`
  (`encanador|eletricista|pintor|jardineiro|seguranca|limpeza|outro`),
  `phone`, `email`, `notes`, `created_by` (FK `users`), `created_at`,
  `updated_at`.
- Módulo `internal/providers/` — CRUD completo e simples (síndico).
- Rotas: `GET /providers`, `GET /providers/new`, `POST /providers`,
  `GET /providers/{id}/edit`, `POST /providers/{id}`,
  `POST /providers/{id}/delete` (hard delete no MVP), todas com
  `RequireRole(syndic)`.
- Testes: store + service + handler.

**Pronto quando:** `make check` verde; CRUD funcional no navegador via
`make run`.

## Fase 2 — Contas a pagar/receber (migração 0006)

- Tabela única `ledger_entries` (um módulo `internal/finances/`):
  `id`, `kind` (`payable|receivable`), `description`, `category`
  (`condominio|agua|energia|manutencao|salario|imposto|outro`),
  `amount NUMERIC(12,2)`, `due_date DATE`, `status` (`pending|settled`),
  `settled_at TIMESTAMPTZ`, `notes`, `created_by`, `created_at`, `updated_at`.
- Regras de negócio:
  - síndico lança/edita/exclui; `pending → settled` registra `settled_at` e é
    irreversível no MVP (lançamento liquidado não pode ser editado/excluído);
  - vencido = `status = pending` e `due_date < hoje` (badge vermelho);
  - resumo no topo: total a pagar pendente, total a receber pendente, saldo
    (recebido − pago).
- Rotas: `GET /finances`, `POST /finances`, `GET /finances/{id}/edit`,
  `POST /finances/{id}`, `POST /finances/{id}/settle`,
  `POST /finances/{id}/delete`, todas `RequireRole(syndic)`.
- Testes: service (transição de status, regra de vencimento) + store + handler.

**Pronto quando:** `make check` verde; lançar pagar/receber, liquidar e ver o
resumo no navegador.

## Fase 3 — Eleições (finalizar trabalho em andamento)

- Revisar o diff não commitado (`internal/election/`, `internal/model/election.go`,
  migrações 0003/0004) e **commitar**.
- Conferir a troca de papel já implementada em `SettleTransitions`
  (`internal/election/store.go`): no fim do período de transição o vencedor
  vira `syndic` e o síndico anterior volta a `owner`. É exatamente a política
  de sucessão: primeiro síndico manual (Fase 0), próximos via eleição.
- Endurecer com a Fase 0: `RequireRole(syndic)` em criar eleição, abrir votação,
  fechar; `RequireRole(owner)` em votar (tenant não vota/candidata — ver comentário
  em `internal/model/user.go`); candidatura de `owner` e do síndico atual
  (reeleição — `canRun` em `internal/election/service.go`).
- Conferir o `settler.go` (transição automática `transition → closed`) e os
  testes de service/store.
- O fluxo já cobre "iniciar/finalizar o período de eleição" com ciclo completo:
  `nomination → voting → transition → closed`.

**Pronto quando:** `make check` verde com o módulo commitado; um morador
`owner` consegue candidatar-se, votar e ver o resultado final; ao encerrar o
período de transição, o vencedor aparece como `syndic` e o síndico anterior
como `owner` (verificado por teste de store).

## Fase 4 — Tickets (migração 0007)

- Tabelas `tickets` e `ticket_comments`:
  - `tickets`: `id`, `title`, `description`, `category`
    (`reparo|infraestrutura|seguranca|limpeza|outro`), `status`
    (`open|in_progress|resolved|closed`), `priority` (`low|medium|high`),
    `created_by` (FK `users`), `assigned_to` (FK `users`, nullable),
    `created_at`, `updated_at`, `closed_at`.
  - `ticket_comments`: `id`, `ticket_id` (FK, CASCADE), `user_id`, `body`,
    `created_at`.
- Regras: `owner`/`tenant` criam e comentam; síndico lista todos, muda status,
  atribui (`assigned_to`) e fecha (`resolved`/`closed`). Fechado é somente
  leitura.
- Módulo `internal/tickets/`; rotas:
  - morador: `GET /tickets`, `GET /tickets/new`, `POST /tickets`,
    `GET /tickets/{id}`, `POST /tickets/{id}/comments`;
  - síndico: `POST /tickets/{id}/status`, `POST /tickets/{id}/assign`.
- Testes: service (transições válidas por papel) + store + handler.

**Pronto quando:** `make check` verde; morador abre ticket, síndico responde,
atribui e fecha.

## Fase 5 — Comunicações (migração 0008) — backbone de notificação

- Tabela `announcements`: `id`, `title`, `body`, `audience`
  (`all|owners|tenants|individual`), `user_id` (FK, nullable — usado quando
  `audience = individual`), `created_by` (FK, síndico), `created_at`.
- Módulo `internal/announcements/`:
  - síndico: criar aviso individual (escolhe morador) ou coletivo (todos /
    owners / tenants) — `GET /announcements`, `GET /announcements/new`,
    `POST /announcements`;
  - morador: inbox no dashboard listando os avisos que lhe cabem
    (audience cobre o papel dele ou `individual` com `user_id` dele), com
    badge de não lido via HTMX.
- Vira o backbone: a Fase 7 posta um `announcement` automaticamente ao criar
  reunião (convite coletivo).
- Testes: service (filtro de audiência por papel/usuário) + store + handler.

**Pronto quando:** `make check` verde; síndico envia aviso coletivo e
individual, morador vê no inbox com marcação de não lido.

## Fase 6 — Pautas (migração 0009)

- Tabela `agenda_items`: `id`, `title`, `description`, `proposed_by` (FK
  `users`, nullable = criada pelo síndico), `status`
  (`proposed|approved|rejected|scheduled|discussed`), `meeting_id` (FK
  `meetings`, nullable — preenchida na Fase 7), `created_at`, `decided_at`.
- Módulo `internal/agenda/`:
  - morador propõe pauta (`POST /agenda/propose`);
  - síndico aprova/rejeita (`POST /agenda/{id}/approve`,
    `POST /agenda/{id}/reject`) e pode criar diretamente
    (`GET /agenda/new`, `POST /agenda`);
  - lista pública com status (`GET /agenda`).
- Testes: service (transições de status) + store + handler.

**Pronto quando:** `make check` verde; morador propõe, síndico aprova e a pauta
aparece na lista com status.

## Fase 7 — Reuniões (migração 0010)

- Tabela `meetings`: `id`, `title`, `starts_at TIMESTAMPTZ`, `location` (local
  físico ou link), `status` (`scheduled|done|cancelled`), `created_by`,
  `created_at`, `ended_at`. Pautas aprovadas entram na pauta da reunião via
  `agenda_items.meeting_id` (Fase 6).
- Módulo `internal/meetings/`:
  - síndico: `GET /meetings`, `GET /meetings/new`, `POST /meetings` (multi-select
    de pautas aprovadas → marca `scheduled`), `GET /meetings/{id}`,
    `POST /meetings/{id}/status` (done/cancelled);
  - ao criar, publica automaticamente um convite coletivo via Fase 5
    (`audience = all`);
  - morador: `GET /meetings`, `GET /meetings/{id}` (somente leitura).
- Testes: service (criar reunião anexa pautas e dispara convite) + store +
  handler.

**Pronto quando:** `make check` verde; síndico agenda reunião com pautas
anexas, moradores recebem o convite no inbox e veem a lista de reuniões.

---

## Padrão de cada fase (checklist de definição de pronto)

1. Migração versionada (`NNNN_nome.{up,down}.sql`) + tipos em
   `internal/model/` quando o domínio é compartilhado.
2. Módulo completo: `module.go` (Deps + New + RegisterRoutes),
   `handler.go` (fino), `service.go` (regras de negócio, sem HTTP/SQL),
   `store.go` (`database/sql` + SQL puro + erros de domínio),
   `errors.go`, `templates/`.
3. Rotas protegidas: `web.RequireAuth` + `web.RequireRole` conforme o papel.
4. Forms com CSRF (`web.ValidCSRF`) e re-render via HTMX / redirect.
5. Wire em `internal/app/app.go` + links na nav/base layout.
6. Testes: store + service + handler em `*_test.go` ao lado do código.
7. `gofmt -w .`, `templ generate`, `make css`, `make check` verde.
8. Smoke test manual com `make db-up` + `make run`.

## Fora de escopo (próximas iterações)

- Relatórios financeiros (extrato mensal, export CSV) e categorização avançada.
- Multi-condomínio/unidades (hoje: condomínio único).
- Anexos em tickets e e-mail/push real fora do app (inbox in-app no MVP).
- Ata de reunião, presença/RSVP e votação online em assembleia.
- Fila de notificação assíncrona (hoje o envio é síncrono ao criar o registro).
