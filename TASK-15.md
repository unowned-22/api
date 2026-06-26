# TASK-15. Разбивка `MessengerService` на фокусные сервисы

Technical Task Specification for AI Coding Agent
Backend (Go) · Июнь 2026

---

## Контекст

`internal/service/messenger_service.go` — 709 строк, 39 публичных методов,
10 зависимостей в конструкторе. Сервис покрывает четыре концептуально разные
области: управление конверсациями, работу с сообщениями, черновики и
приватность/блокировки. Дальнейший рост фичей (реакции, mentions, read
receipts через outbox, scheduled delivery) сделает файл нечитаемым.

Цель задачи — разбить `MessengerService` на три фокусных сервиса без изменения
внешнего поведения и без поломки хендлеров или воркеров.

---

## Целевая структура

### Три новых сервиса

**`ConversationService`** — жизненный цикл конверсаций и членство:

```
GetOrCreateDirect, CreateGroup, CreateChannel
GetConversation, ListConversations
ArchiveConversation, UnarchiveConversation
AddMembers, RemoveMember, LeaveConversation, Subscribe
GenerateInviteLink, JoinByInviteLink, RevokeInviteLink
SetDisappearingTimer
```

Зависимости: `convRepo`, `memberRepo`, `privacyRepo`, `friendSvc`

---

**`MessageService`** — работа с сообщениями:

```
SendMessage, ScheduleMessage, ReplyToMessage, ForwardMessage
EditMessage, DeleteMessage
ListMessages, SearchMessages
PinMessage, UnpinMessage
LikeMessage, UnlikeMessage
MarkRead, MarkDelivered
SaveDraft, GetDraft, DeleteDraft
```

Зависимости: `convRepo`, `msgRepo`, `memberRepo`, `privacyRepo`,
`draftRepo`, `storage`, `publicBucket`, `eventBus`

---

**`MessengerPrivacyService`** — блокировки и настройки приватности:

```
CanMessage
BlockUser, UnblockUser, ListBlocked
GetPrivacySettings, UpdatePrivacySettings
```

Зависимости: `privacyRepo`, `friendSvc`

---

### Интерфейсы в доменном слое

Разбить `messenger.Service` в `internal/domain/messenger/interfaces.go` на три
соответствующих интерфейса:

```go
type ConversationService interface { ... }
type MessageService interface { ... }
type PrivacyService interface { ... }
```

Сохранить сводный интерфейс `Service` для обратной совместимости на период
перехода — он встраивает все три:

```go
// Service объединяет все под-сервисы мессенджера. Используется там, где
// хендлер или воркер исторически получал один объект. Новый код должен
// зависеть от конкретного под-интерфейса, а не от Service.
type Service interface {
    ConversationService
    MessageService
    PrivacyService
}
```

Это позволяет не менять `MessengerHandler` и воркеры в рамках данного таска —
они продолжают получать `messenger.Service`, а конкретные реализации
удовлетворяют всем трём интерфейсам через фасад (см. ниже).

---

### Фасад для обратной совместимости

Создать `internal/service/messenger_facade.go`:

```go
// MessengerFacade реализует messenger.Service, делегируя каждый вызов
// соответствующему под-сервису. Используется в bootstrap как единственная
// точка сборки для хендлеров, которые пока зависят от полного Service.
// По мере того как хендлеры переводятся на конкретные под-интерфейсы,
// фасад выводится из употребления.
type MessengerFacade struct {
    Conv    messenger.ConversationService
    Msg     messenger.MessageService
    Privacy messenger.PrivacyService
}
```

Каждый метод фасада — однострочный делегат:

```go
func (f *MessengerFacade) SendMessage(ctx context.Context, ...) (*messenger.Message, error) {
    return f.Msg.SendMessage(ctx, ...)
}
```

Фасад не содержит бизнес-логики. Compile-time check обязателен:

```go
var _ messenger.Service = (*MessengerFacade)(nil)
```

---

## Что менять

### `internal/domain/messenger/interfaces.go`

- Добавить `ConversationService`, `MessageService`, `MessengerPrivacyService`.
- Оставить `Service` как сводный интерфейс (три embed'а).
- Не удалять ни один метод из `Service`.

### `internal/service/`

Создать три файла:

| Файл | Сервис |
|---|---|
| `conversation_service.go` | `ConversationService` |
| `message_service.go` | `MessageService` |
| `messenger_privacy_service.go` | `MessengerPrivacyService` |

Переместить методы из `messenger_service.go` по принадлежности.
`messenger_service.go` удалить после переноса всех методов.

Создать `messenger_facade.go` с `MessengerFacade`.

Каждый сервис должен иметь compile-time check своего интерфейса:

```go
var _ messenger.ConversationService = (*ConversationService)(nil)
```

### `internal/bootstrap/services.go`

Собрать три сервиса и обернуть фасадом:

```go
convSvc    := service.NewConversationService(...)
msgSvc     := service.NewMessageService(...)
privacySvc := service.NewMessengerPrivacyService(...)

messengerSvc := &service.MessengerFacade{
    Conv:    convSvc,
    Msg:     msgSvc,
    Privacy: privacySvc,
}
```

`Services.Messenger` остаётся типа `messenger.Service` — хендлеры не трогаем.

### Тесты

`messenger_service_test.go` разбить по тому же принципу:

- `conversation_service_test.go`
- `message_service_test.go`
- `messenger_privacy_service_test.go`

Существующие тесты из TASK-14 (проверка `draftRepo.Delete`, `UpdateLastMessage`,
`ListMembers call count`, block check) переехать в `message_service_test.go`.

---

## Чего не делать в рамках этого таска

- **Не менять** `MessengerHandler` — он продолжает зависеть от `messenger.Service`.
- **Не менять** воркеры (`DisappearingMessageWorker`, `ScheduledMessageWorker`).
- **Не менять** domain-репозитории и их интерфейсы.
- **Не добавлять** новую функциональность — только перемещение кода.
- **Не трогать** `messenger.Service` интерфейс — только добавить три новых.

---

## Порядок выполнения

1. Добавить три новых интерфейса в `interfaces.go`, оставив `Service`.
2. Создать три сервиса с перенесёнными методами, добавить compile-time checks.
3. Убедиться что `go build ./...` зелёный.
4. Создать `MessengerFacade`, подключить в `bootstrap`.
5. Удалить `messenger_service.go`.
6. Перенести/разбить тесты.
7. `go test ./internal/service/... ./internal/bootstrap/...` зелёный.

---

## Критерии приёмки

- `go build ./...` без ошибок.
- `go test ./...` без регрессий.
- `messenger_service.go` отсутствует.
- Каждый из трёх новых сервисов зависит только от тех репозиториев,
  которые ему нужны (проверить конструктор).
- `MessengerFacade` не содержит ни одной строки бизнес-логики.
- `MessengerHandler` не изменён (проверить `git diff`).