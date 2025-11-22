---
title: "Контекст и горутины"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

Горутины — это лёгкие потоки выполнения, управляемые runtime Go. Они намного легче системных потоков, и ты можешь запустить тысячи или даже миллионы горутин на одной машине без критических потерь производительности.

Когда ты запускаешь горутину с помощью ключевого слова `go`, он запускается асинхронно и параллельно с остальным кодом.

```go
go функция()  // запустить функцию в отдельной горутине
```

Главный процесс (main goroutine) будет ждать завершения всех горутин. Если main завершится раньше, все остальные горутины будут прерваны.

## Почему нужен контекст?

Когда у тебя есть одна или несколько горутин, работающих над какой-то задачей, тебе нужно:

1. **Управлять их жизненным циклом** — знать, когда их остановить
2. **Устанавливать сроки** — например, операция не должна длиться более 5 секунд
3. **Передавать информацию** — например, ID пользователя или авторизационный токен
4. **Сигнализировать об отмене** — когда клиент отключился или пришла команда остановиться

Контекст решает все эти проблемы.

## Что такое context.Context?

`context.Context` — это ***интерфейс*** (запомни это!), который несёт в себе:

```go
type Context interface {
 Deadline() (deadline time.Time, ok bool)
 Done() <-chan struct{}
 Err() error
 Value(key any) any
}
```

Вот буквально то, что находится у него внутри

- Сигнал об отмене операции
- Дедлайн (deadline) — момент времени, после которого операция должна завершиться
- Значения, связанные с запросом (request-scoped values)

Context безопасен для одновременного использования несколькими горутинами. Следовательно, когда задают вопрос про примитивы синхронизации, разумно добавить: "Ну есть еще контекст, правда внутри него канал, но уместно выделить отдельно".

## Основные функции для создания контекста

### context.Background()

Это корневой контекст. Он никогда не отменяется, не имеет дедлайна и не содержит значений.

```go
ctx := context.Background()
```

Используй `Background()` когда:

- Ты в главной функции, обработчике HTTP запроса
- Тебе нужна **точка старта для иерархии контекстов**

### context.TODO()

Это используют, когда ещё не знают, какой контекст использовать, или планируют добавить его позже. Пару раз в коде видел. Работа с ним как с Background().

```go
ctx := context.TODO()  // будет заменён позже
```

### context.WithCancel()

Создаёт новый контекст, который можно отменить вручную с помощью функции cancel.

```go
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()  // всегда отменяй контекст
```

Когда ты вызываешь `cancel()`, все горутины, которые ждут на `ctx.Done()`, получают сигнал.

**Пример:**

Можешь его переписать ручками у убедиться, что выведется.

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    go worker(ctx)

    time.Sleep(2 * time.Second)
    cancel()  // остановить рабочую горутину

    time.Sleep(1 * time.Second)  // дать время на завершение
}

func worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            fmt.Println("Worker остановлен")
            return
        default:
            fmt.Println("Worker работает...")
            time.Sleep(500 * time.Millisecond)
        }
    }
}
```

### context.WithTimeout()

Создаёт контекст, который автоматически отменяется через указанный промежуток времени.

```go
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel()
```

**Пример:**

```go
func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    result := make(chan string)

    go func() {
        time.Sleep(3 * time.Second)  // операция длится 3 секунды
        result <- "готово"
    }()

    select {
    case <-ctx.Done():
        fmt.Println("Превышено время ожидания:", ctx.Err())
    case res := <-result:
        fmt.Println(res)
    }
}
```

### context.WithDeadline()

Похоже на `WithTimeout()`, но ты указываешь абсолютное время (не длительность).

В этом примере взяли текущий момент времени и прибавили к нему 5 сек.

```go
deadline := time.Now().Add(5 * time.Second)
ctx, cancel := context.WithDeadline(parentCtx, deadline)
defer cancel()
```

### context.WithValue()

Добавляет значение к контексту. Используй его для передачи request-scoped данных.

Это антипаттерн! Надо только для какого-нибудь трейсинга. Либо передавать id пользователя, данные авторизации. Для остального нафиг не надо.

```go
ctx := context.WithValue(parentCtx, userIDKey, 12345)
```

Получаем значение:

```go
userID := ctx.Value(userIDKey).(int)
```

## Как работать с контекстом в горутинах?

### Проверка отмены через ctx.Done()

`Done()` возвращает канал, который закрывается когда контекст отменяется или истекает его дедлайн.

```go
func worker(ctx context.Context) {
    select {
    case <-ctx.Done():
        fmt.Println("Контекст отменён или истёк дедлайн")
        return
    case <-time.After(1 * time.Second):
        fmt.Println("Работаю...")
    }
}
```

### Проверка ошибки через ctx.Err()

`Err()` возвращает ошибку отмены контекста. Есть две основные ошибки:

- `context.Canceled` — контекст был отменён явно через `cancel()`
- `context.DeadlineExceeded` — истёк дедлайн

```go
func worker(ctx context.Context) {
    <-ctx.Done()
    err := ctx.Err()

    if err == context.Canceled {
        fmt.Println("Кто-то явно отменил контекст")
    } else if err == context.DeadlineExceeded {
        fmt.Println("Истёк дедлайн")
    }
}
```

### Проверка дедлайна через ctx.Deadline()

Используй это если нужно узнать, сколько времени осталось до дедлайна.

```go
deadline, ok := ctx.Deadline()
if ok {
    remaining := time.Until(deadline)
    fmt.Println("Времени осталось:", remaining)
}
```

## Лучшие практики

### 1. Всегда передавай контекст первым параметром

```go
func DoSomething(ctx context.Context, data string) error {
    // правильно
}

func DoSomething(data string, ctx context.Context) error {
    // неправильно
}
```

### 2. Никогда не сохраняй контекст в структуре

```go
// Плохо
type Handler struct {
    ctx context.Context
}

// Хорошо
func (h *Handler) Do(ctx context.Context) error {
    // ...
}
```

Причина: контекст должен течь вниз по цепочке вызовов, а не сохраняться.

### 3. Всегда вызывай defer cancel()

```go
ctx, cancel := context.WithCancel(parent)
defer cancel()  // обязательно!
```

Без этого могут утечь ресурсы.

### 4. Используй context.Value осторожно

`Value()` предназначен только для request-scoped данных, не для передачи параметров функций.

```go
// Хорошо: передача ID пользователя из HTTP контекста
ctx = context.WithValue(ctx, userIDKey, userID)

// Плохо: передача опциональных параметров
ctx = context.WithValue(ctx, retryCountKey, 3)  // это должно быть параметром функции
```

### 5. Всегда проверяй ctx.Done()

В долгоживущих операциях регулярно проверяй, не отменён ли контекст.

```go
func process(ctx context.Context) error {
    for item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // обработать item
        }
    }
    return nil
}
```

### 6. Не передавай nil контекст

```go
// Плохо
function(nil)

// Хорошо
function(context.Background())
```

## Типичные паттерны использования

### Паттерн 1: Отмена при получении сигнала

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT)

    go func() {
        <-sigChan
        cancel()  // отменить при Ctrl+C
    }()

    server.Serve(ctx)
}
```

### Паттерн 2: Таймаут для HTTP запроса

```go
func fetchData(url string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, _ := ioutil.ReadAll(resp.Body)
    return string(body), nil
}
```

### Паттерн 3: Передача значения вниз по цепочке

```go
type requestIDKey struct{}

func main() {
    ctx := context.WithValue(context.Background(), requestIDKey{}, "req-123")
    handler(ctx)
}

func handler(ctx context.Context) {
    logger(ctx)
}

func logger(ctx context.Context) {
    requestID := ctx.Value(requestIDKey{})
    fmt.Println("Request ID:", requestID)
}
```

### Паттерн 4: Согласование нескольких горутин

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    var wg sync.WaitGroup

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            worker(ctx, id)
        }(i)
    }

    time.Sleep(2 * time.Second)
    cancel()  // остановить все рабочие горутины

    wg.Wait()  // ждать завершения всех
    fmt.Println("Все горутины закончили")
}

func worker(ctx context.Context, id int) {
    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d остановлен\n", id)
            return
        default:
            fmt.Printf("Worker %d работает\n", id)
            time.Sleep(500 * time.Millisecond)
        }
    }
}
```

## Распространённые ошибки

### Ошибка 1: Забыли вызвать cancel()

```go
// Плохо - утечка ресурсов
func bad() {
    ctx, cancel := context.WithCancel(context.Background())
    // cancel никогда не вызывается
}

// Хорошо
func good() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
}
```

### Ошибка 2: Игнорирование ctx.Done()

```go
// Плохо - горутина будет работать даже после отмены
func bad(ctx context.Context) {
    for {
        doWork()
    }
}

// Хорошо
func good(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            doWork()
        }
    }
}
```

### Ошибка 3: Сохранение контекста в структуре

```go
// Плохо
type MyService struct {
    ctx context.Context
}

// Хорошо
func (s *MyService) Do(ctx context.Context) error {
    // использовать ctx как параметр
}
```

### Ошибка 4: Передача nil вместо контекста

```go
// Плохо
function(nil)

// Хорошо
function(context.Background())
```

### Ошибка 5: Перелезание через context.WithValue

```go
// Плохо - слишком много данных в контексте
ctx = context.WithValue(ctx, "name", "John")
ctx = context.WithValue(ctx, "age", 30)
ctx = context.WithValue(ctx, "email", "john@example.com")

// Лучше - передай эти данные явно через параметры
```

## Часто задаваемые вопросы на собеседованиях

### В чём разница между context.Background() и context.TODO()?

**Ответ:**

`context.Background()` — это корневой контекст, который никогда не отменяется. Используй его как стартовую точку для всех контекстов в приложении.

`context.TODO()` — это заполнитель для случаев, когда ты ещё не решил, какой контекст использовать, или планируешь добавить правильный контекст позже.

На практике `Background()` используется в 99% случаев. `TODO()` — это сигнал для кода-ревьюера, что здесь нужно ещё подумать.

### Что происходит, если я вызову cancel() несколько раз?

**Ответ:**

Ничего страшного. Вызов cancel() несколько раз безопасен. После первого вызова контекст отменяется, все последующие вызовы просто ничего не делают.

```go
cancel()  // закрывает ctx.Done()
cancel()  // безопасно, ничего не происходит
```

### Как передать контекст при запуске HTTP сервера?

**Ответ:**

Используй `http.Server` с методом `Serve()` или `ListenAndServe()` вместе с контекстом для graceful shutdown:

```go
server := &http.Server{Addr: ":8080"}

go func() {
    server.ListenAndServe()
}()

<-sigChan  // ждём сигнала
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
server.Shutdown(ctx)  // graceful shutdown
```

### Может ли контекст быть использован в нескольких горутинах одновременно?

**Ответ:**

Да, контекст полностью потокобезопасен. Ты можешь передать один контекст сотням или тысячам горутин, и все они будут получать сигнал отмены одновременно.

```go
ctx, cancel := context.WithCancel(context.Background())

for i := 0; i < 1000; i++ {
    go worker(ctx)  // все рабочие делят один контекст
}

cancel()  // все 1000 рабочих получат сигнал
```

### Как обработать ошибку timeout в отличие от явной отмены?

**Ответ:**

Используй `ctx.Err()` для проверки причины отмены:

```go
<-ctx.Done()
if ctx.Err() == context.Canceled {
    // явная отмена
} else if ctx.Err() == context.DeadlineExceeded {
    // таймаут
}
```

### Нужно ли всегда вызывать defer cancel()?

**Ответ:**

Да, это обязательно. Без defer cancel() ты оставляешь горутину, ассоциированную с контекстом, работающей, даже после того как она больше не нужна. Это приводит к утечке памяти и ресурсов.

```go
ctx, cancel := context.WithCancel(parent)
defer cancel()  // ВСЕГДА!
```

### Может ли родительский контекст отменить дочерний контекст?

**Ответ:**

Да, если отменить родительский контекст, все дочерние контексты тоже будут отменены.

```go
parent, parentCancel := context.WithCancel(context.Background())
child, childCancel := context.WithCancel(parent)

parentCancel()  // отменяет и parent, и child

// child.Done() будет закрыт
```

### Что произойдёт, если я используюcontext.WithTimeout() с нулевой или отрицательной длительностью?

**Ответ:**

Контекст будет немедленно отменён (или уже будет отменён в момент создания). Это может быть полезно для тестирования, но в продакшене это ошибка.

```go
ctx, cancel := context.WithTimeout(parent, -1 * time.Second)
defer cancel()

<-ctx.Done()  // закроется немедленно
```

### Как передать контекст в database.sql запросе?

**Ответ:**

Используй `QueryContext()` или `ExecContext()`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

rows, err := db.QueryContext(ctx, "SELECT * FROM users")
```

### Когда следует использовать context.Value()?

**Ответ:**

`context.Value()` предназначен только для данных, связанных с запросом (request-scoped data), например:

- ID пользователя
- Request ID для трейсинга
- Авторизационный токен
- Информация о сессии

НЕ используй его для:

- Конфигурации приложения
- Опциональных параметров функций
- Долгоживущих объектов

Лучше передавай эти данные явно через параметры функций.

## Практические задачи

### Задача 1: Простой timeout

Напиши функцию, которая получает данные с URL с таймаутом 10 секунд. Если таймаут превышен, функция должна вернуть ошибку.

```go
func fetchWithTimeout(url string) ([]byte, error) {
    // твой код здесь
}
```

### Задача 2: Graceful shutdown

Создай простой HTTP сервер, который слушает сигналы SIGINT и gracefully шатаун в течение 5 секунд.

```go
func main() {
    // твой код здесь
}
```

### Задача 3: Отмена по требованию

Напиши программу с 5 рабочими горутинами. Главная горутина должна после 3 секунд отменить контекст, и все рабочие должны завершиться.

```go
func main() {
    // твой код здесь
}
```

### Задача 4: Request ID в контексте

Создай небольшое веб-приложение, которое для каждого запроса генерирует уникальный Request ID, добавляет его в контекст и логирует все операции с этим ID.

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // твой код здесь
}
```

### Задача 5: Комбинирование timeout и cancel

Создай функцию, которая:

1. Принимает контекст (может быть с дедлайном)
2. Дополнительно добавляет собственный timeout 30 секунд
3. Использует тот дедлайн, который наступит раньше
4. Делает несколько HTTP запросов

```go
func processRequests(ctx context.Context, urls []string) error {
    // твой код здесь
}
```

### Задача 6: Обработка ошибок контекста

Напиши функцию, которая обрабатывает три случая:

1. Контекст был явно отменён
2. Превышен таймаут
3. Операция завершена успешно

```go
func doWork(ctx context.Context) error {
    // твой код здесь
}
```

### Задача 7: Pipeline с контекстом

Создай pipeline из трёх этапов (stage 1 → stage 2 → stage 3), где каждый этап работает в отдельной горутине. Отмена контекста должна остановить весь pipeline.

```go
func stage1(ctx context.Context, input <-chan int) <-chan int {
    // твой код здесь
}

func stage2(ctx context.Context, input <-chan int) <-chan int {
    // твой код здесь
}

func stage3(ctx context.Context, input <-chan int) <-chan int {
    // твой код здесь
}
```

### Задача 8: Worker pool с контекстом

Создай worker pool из N рабочих, которые обрабатывают задачи из канала. Когда контекст отменяется, все рабочие должны завершиться и очистить ресурсы.

```go
type WorkerPool struct {
    workers int
}

func (wp *WorkerPool) Start(ctx context.Context, jobs <-chan Job) {
    // твой код здесь
}
```

### Задача 9: Deadline vs Timeout

Объясни разницу между `WithDeadline()` и `WithTimeout()`. Когда бы ты использовал каждый из них?

### Задача 10: Context leak detection

Напиши тест, который проверяет, что функция правильно очищает контекст (вызывает cancel()).
