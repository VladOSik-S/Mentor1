---
title: "Fan-In, Fan-Out паттерны"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

Fan-In и Fan-Out — это крайне часто встречаемые паттерны конкурентности в Go, которые позволяют эффективно распараллеливать обработку данных и агрегировать результаты. Это база для построения пайплайнов, они часто встречаются в production-коде.

## Основные Концепции

### Fan-Out

Fan-Out — это паттерн, при котором один источник данных распределяет задачи между несколькими горутинами для параллельной обработки. Это напоминает разветвление потока: один входной канал читается несколькими горутинами одновременно.

**Ключевые характеристики:**

- Несколько горутин читают из одного канала
- Задачи обрабатываются параллельно
- Порядок обработки не гарантируется
- Используется для CPU-bound и I/O-bound задач

**Когда применять:**

- Задачи независимы друг от друга
- Порядок обработки не важен
- Нужно увеличить throughput
- Есть ресурсы для параллельной обработки

### Простой Fan-Out

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

// generator создает канал и отправляет в него данные
// обратите внимание что это отдельная, начальная функция
func generator(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            out <- n
        }
    }()
    return out
}

// worker обрабатывает данные из входного канала
// сначала посмотрите на main
func worker(id int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        fmt.Printf("Worker %d обрабатывает задачу %d\n", id, j)
        time.Sleep(time.Millisecond * 500) // имитация работы
        results <- j * 2
    }
}

func main() {
    jobs := generator(1, 2, 3, 4, 5, 6, 7, 8)
    results := make(chan int, 8)

    // Fan-Out: запускаем 3 воркера
    numWorkers := 3
    var wg sync.WaitGroup

    for i := 1; i <= numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            // пока в jobs будут задачи, какой-то worker
            // из зарегистрированных трех ее возьмет
            worker(workerID, jobs, results)
        }(i)
    }

    // Закрываем results после завершения всех воркеров
    go func() {
        wg.Wait()
        close(results)
    }()

    // Читаем результаты
    for result := range results {
        fmt.Printf("Результат: %d\n", result)
    }
}
```

### Fan-In

Fan-In — это паттерн, обратный Fan-Out. Он объединяет данные из нескольких каналов в один выходной канал. Это мультиплексирование нескольких потоков данных.

**Ключевые характеристики:**

- Несколько входных каналов объединяются в один
- Использует select для неблокирующего чтения
- Агрегирует результаты от множества воркеров
- Обеспечивает синхронизацию результатов

**Когда применять:**

- Нужно собрать результаты от нескольких воркеров
- Требуется централизованная обработка результатов
- Необходима координация между горутинами

### Простой Fan-In

```go
package main

import (
    "fmt"
    "time"
)

func producer(id int, ch chan<- int) {
    for i := 0; i < 5; i++ {
        ch <- id*10 + i
        time.Sleep(time.Millisecond * 100)
    }
    close(ch)
}

// fanIn объединяет несколько каналов в один
func fanIn(channels ...<-chan int) <-chan int {
    out := make(chan int)

    go func() {
        defer close(out)

        // Используем select для чтения из всех каналов
        for {
            for _, ch := range channels {
                select {
                case val, ok := <-ch:
                    if ok {
                        out <- val
                    }
                default:
                    // Канал пуст, переходим к следующему
                }
            }

            // Проверяем, все ли каналы закрыты
            allClosed := true
            for _, ch := range channels {
                select {
                case _, ok := <-ch:
                    if ok {
                        allClosed = false
                    }
                default:
                }
            }
            if allClosed {
                break
            }
        }
    }()

    return out
}

func main() {
    ch1 := make(chan int)
    ch2 := make(chan int)
    ch3 := make(chan int)

    go producer(1, ch1)
    go producer(2, ch2)
    go producer(3, ch3)

    // Fan-In: объединяем три канала
    merged := fanIn(ch1, ch2, ch3)

    for val := range merged {
        fmt.Println("Получено:", val)
    }
}
```

## Продвинутая Реализация с WaitGroup и Context

### Fan-Out/Fan-In Pipeline

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Stage 1: Generator
func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 2: Fan-Out - несколько воркеров обрабатывают данные
func square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 3: Fan-In - объединяем результаты
func merge(ctx context.Context, channels ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup

    // Запускаем горутину для каждого канала
    multiplex := func(ch <-chan int) {
        defer wg.Done()
        for val := range ch {
            select {
            case out <- val:
            case <-ctx.Done():
                return
            }
        }
    }

    wg.Add(len(channels))
    for _, ch := range channels {
        go multiplex(ch)
    }

    // Закрываем выходной канал после завершения всех горутин
    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Генерируем данные
    in := generate(ctx, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

    // Fan-Out: создаем несколько обработчиков
    c1 := square(ctx, in)
    c2 := square(ctx, in)
    c3 := square(ctx, in)

    // Fan-In: объединяем результаты
    for result := range merge(ctx, c1, c2, c3) {
        fmt.Println("Результат:", result)
    }
}
```

### Worker Pool Pattern

Рекомендуется отдельно прочитать про worker pool до рассмотрения этого примера

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type Job struct {
    ID   int
    Data string
}

type Result struct {
    Job       Job
    Output    string
    ProcessedBy int
}

// WorkerPool реализует паттерн Fan-Out/Fan-In
type WorkerPool struct {
    numWorkers int
    jobs       chan Job
    results    chan Result
    wg         sync.WaitGroup
}

func NewWorkerPool(numWorkers, jobQueueSize int) *WorkerPool {
    return &WorkerPool{
        numWorkers: numWorkers,
        jobs:       make(chan Job, jobQueueSize),
        results:    make(chan Result, jobQueueSize),
    }
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
    defer wp.wg.Done()

    for {
        select {
        case job, ok := <-wp.jobs:
            if !ok {
                return
            }

            // Обработка задачи
            time.Sleep(time.Millisecond * 100)
            result := Result{
                Job:         job,
                Output:      fmt.Sprintf("Обработано: %s", job.Data),
                ProcessedBy: id,
            }

            select {
            case wp.results <- result:
            case <-ctx.Done():
                return
            }

        case <-ctx.Done():
            return
        }
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    // Fan-Out: запускаем воркеров
    wp.wg.Add(wp.numWorkers)
    for i := 1; i <= wp.numWorkers; i++ {
        go wp.worker(ctx, i)
    }
}

func (wp *WorkerPool) Submit(job Job) {
    wp.jobs <- job
}

func (wp *WorkerPool) Close() {
    close(wp.jobs)
}

func (wp *WorkerPool) Wait() {
    wp.wg.Wait()
    close(wp.results)
}

func (wp *WorkerPool) Results() <-chan Result {
    return wp.results
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    pool := NewWorkerPool(3, 10)
    pool.Start(ctx)

    // Отправляем задачи
    go func() {
        for i := 1; i <= 10; i++ {
            pool.Submit(Job{
                ID:   i,
                Data: fmt.Sprintf("task-%d", i),
            })
        }
        pool.Close()
    }()

    // Собираем результаты (Fan-In)
    go pool.Wait()

    for result := range pool.Results() {
        fmt.Printf("Job %d обработана воркером %d: %s\n",
            result.Job.ID, result.ProcessedBy, result.Output)
    }
}
```

## Важные Термины и Концепции

### Горутины (Goroutines)

Легковесные потоки выполнения, управляемые Go runtime. Требуют всего несколько килобайт памяти.

### Каналы (Channels)

Механизм коммуникации между горутинами. Обеспечивают синхронизацию и передачу данных.

**Типы каналов:**

- Небуферизованные: блокируют отправителя до получения
- Буферизованные: блокируют только при заполнении буфера
- Однонаправленные: только для чтения или записи

### sync.WaitGroup

Примитив синхронизации для ожидания завершения группы горутин.

**Методы:**

- `Add(delta int)` — увеличивает счетчик
- `Done()` — уменьшает счетчик на 1
- `Wait()` — блокируется до обнуления счетчика

### context.Context

Механизм для управления временем жизни горутин, отмены операций и передачи значений.

**Основные функции:**

- `WithCancel()` — создает контекст с ручной отменой
- `WithTimeout()` — создает контекст с таймаутом
- `WithDeadline()` — создает контекст с конечным временем

### Select Statement

Конструкция для работы с несколькими каналами одновременно. Блокируется до готовности одного из случаев.

### Pipeline

Паттерн, где данные проходят через серию этапов обработки, соединенных каналами.

### Multiplexing

Объединение нескольких входных потоков в один выходной.

### Throttling

Ограничение количества одновременно выполняющихся горутин или скорости обработки.

### Race Condition

Ситуация, когда поведение программы зависит от порядка выполнения горутин.

### Goroutine Leak - утечка горутин

Ситуация, когда горутина продолжает работать, но результаты ее работы больше не нужны.

## Best Practices

### 1. Всегда Закрывайте Каналы

Отправитель должен закрывать канал. Это сигнализирует получателям о завершении.

```go
go func() {
    defer close(ch)
    for _, item := range items {
        ch <- item
    }
}()
```

### 2. Используйте Context для Отмены

Context позволяет корректно завершать горутины при таймауте или отмене.

```go
select {
case result <- processedData:
case <-ctx.Done():
    return ctx.Err()
}
```

### 3. Передавайте WaitGroup по Указателю

WaitGroup нельзя копировать после первого использования.

```go
// Правильно
func worker(wg *sync.WaitGroup) {
    defer wg.Done()
    // работа
}

// Неправильно
func worker(wg sync.WaitGroup) {
    defer wg.Done() // копия WaitGroup!
}
```

### 4. Ограничивайте Количество Горутин

Используйте буферизованные каналы или семафоры для throttling.

```go
semaphore := make(chan struct{}, maxGoroutines)

for _, task := range tasks {
    semaphore <- struct{}{} // захватываем слот
    go func(t Task) {
        defer func() { <-semaphore }() // освобождаем слот
        process(t)
    }(task)
}
```

### 5. Избегайте Goroutine Leaks

Всегда обеспечивайте способ завершения горутины.

```go
// Утечка горутины
func leak() <-chan int {
    ch := make(chan int)
    go func() {
        for i := 0; ; i++ {
            ch <- i // заблокируется, если никто не читает
        }
    }()
    return ch
}

// Правильно
func noLeak(ctx context.Context) <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch)
        for i := 0; ; i++ {
            select {
            case ch <- i:
            case <-ctx.Done():
                return
            }
        }
    }()
    return ch
}
```

### 6. Используйте Буферизованные Каналы с Осторожностью

Буферизация может скрыть проблемы синхронизации и увеличить использование памяти.

```go
// Буфер размера = количество воркеров часто оптимален
results := make(chan Result, numWorkers)
```

## Частые Вопросы на Собеседованиях

### 1. В чем разница между Fan-Out и Worker Pool?

**Ответ:** Fan-Out — это общий паттерн распараллеливания, где несколько горутин читают из одного канала. Worker Pool — это конкретная реализация Fan-Out с управлением количеством воркеров, очередью задач и обработкой результатов. Worker Pool добавляет throttling и контроль над использованием ресурсов.

### 2. Как реализовать Fan-In для N каналов?

**Ответ:** Есть два основных подхода:

**Подход 1: Select в цикле**

```go
func fanIn(channels ...<-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for len(channels) > 0 {
            // Динамически строим select
        }
    }()
    return out
}
```

**Подход 2: Горутина на канал с WaitGroup**

```go
func fanIn(channels ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup

    wg.Add(len(channels))
    for _, ch := range channels {
        go func(c <-chan int) {
            defer wg.Done()
            for val := range c {
                out <- val
            }
        }(ch)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

### 3. Когда нужно использовать буферизованные каналы?

**Ответ:** Буферизованные каналы нужны когда:

- Отправитель и получатель работают с разной скоростью
- Нужно избежать блокировки отправителя
- Хотим уменьшить contention между горутинами
- Известно ограниченное количество элементов

Размер буфера часто выбирается равным количеству воркеров или ожидаемому количеству задач.

### 4. Как правильно завершить Worker Pool?

**Ответ:** Нужно:

1. Закрыть канал задач (jobs)
2. Дождаться завершения всех воркеров через WaitGroup
3. Закрыть канал результатов (results)

```go
close(jobs)        // 1. Сигнал воркерам о завершении
wg.Wait()          // 2. Ждем завершения воркеров
close(results)     // 3. Закрываем результаты
```

### 5. В чем разница между конкурентностью и параллелизмом?

**Ответ:**

- **Конкурентность (Concurrency)** — это композиция независимых процессов. Несколько задач могут находиться в процессе выполнения, но не обязательно выполняются одновременно.
- **Параллелизм (Parallelism)** — это одновременное выполнение нескольких задач на разных ядрах процессора.

Go обеспечивает конкурентность через горутины. Параллелизм достигается когда GOMAXPROCS > 1 и доступны физические ядра.

### 6. Что такое happens-before в Go?

**Ответ:** Happens-before — это гарантия упорядочивания операций в конкурентной программе. В Go:

- Отправка в канал happens-before получения из канала
- Закрытие канала happens-before получения нулевого значения
- `wg.Done()` happens-before возврата из `wg.Wait()`
- Чтение переменной в горутине happens-after записи перед `go` statement

### 7. Как обнаружить race condition?

**Ответ:** Используйте встроенный race detector:

```bash
go test -race
go run -race main.go
go build -race
```

Race detector находит одновременный доступ к переменным без синхронизации.

### 8. Можно ли читать из закрытого канала?

**Ответ:** Да, можно. Чтение из закрытого канала сразу возвращает нулевое значение типа канала и false как второе значение:

```go
val, ok := <-ch
if !ok {
    // канал закрыт
}
```

Но отправка в закрытый канал вызывает панику.

### 9. В чем разница между sync.WaitGroup и errgroup.Group?

**Ответ:**

- `sync.WaitGroup` — примитив для ожидания группы горутин без обработки ошибок
- `errgroup.Group` (из `golang.org/x/sync/errgroup`) — обертка над WaitGroup с:
  - Автоматической отменой через context
  - Сбором первой ошибки
  - Встроенным семафором для ограничения количества горутин

```go
g, ctx := errgroup.WithContext(context.Background())
g.SetLimit(10) // максимум 10 горутин

for _, task := range tasks {
    t := task
    g.Go(func() error {
        return process(ctx, t)
    })
}

if err := g.Wait(); err != nil {
    // обработка ошибки
}
```

### 10. Как реализовать graceful shutdown для Worker Pool?

**Ответ:**

```go
type Pool struct {
    jobs    chan Job
    results chan Result
    wg      sync.WaitGroup
    ctx     context.Context
    cancel  context.CancelFunc
}

func (p *Pool) Shutdown(timeout time.Duration) error {
    close(p.jobs) // перестаем принимать новые задачи

    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        close(p.results)
        return nil
    case <-time.After(timeout):
        p.cancel() // форсируем отмену
        return fmt.Errorf("shutdown timeout")
    }
}
```

## Практические Задачи

### Задача 1: Базовый Pipeline

Реализуйте pipeline из трех стадий:

1. Генератор чисел от 1 до N
2. Умножение на 2 (Fan-Out с 3 воркерами)
3. Сложение результатов (Fan-In)

```go
func solution() int {
    ctx := context.Background()

    // Ваш код здесь
    gen := generate(ctx, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

    // Fan-Out
    c1 := multiply(ctx, gen)
    c2 := multiply(ctx, gen)
    c3 := multiply(ctx, gen)

    // Fan-In и сумма
    sum := 0
    for val := range merge(ctx, c1, c2, c3) {
        sum += val
    }

    return sum
}
```

### Задача 2: Rate Limiter

Реализуйте Worker Pool с ограничением скорости обработки (rate limiting):

- Максимум N запросов в секунду
- Используйте `time.Ticker`
- Обеспечьте graceful shutdown

```go
type RateLimitedPool struct {
    // поля
}

func NewRateLimitedPool(workersCount, requestsPerSecond int) *RateLimitedPool {
    // реализация
}
```

### Задача 3: Error Handling

Модифицируйте Worker Pool для обработки ошибок:

- Если воркер возвращает ошибку, остановите весь pool
- Используйте `context.WithCancel`
- Верните первую встреченную ошибку

```go
type Result struct {
    Data interface{}
    Err  error
}

func (wp *WorkerPool) StartWithErrorHandling(ctx context.Context) error {
    // реализация
}
```

### Задача 4: Dynamic Worker Scaling

Реализуйте Worker Pool с динамическим масштабированием:

- Начинаете с minWorkers
- Если очередь растет, добавляете воркеров до maxWorkers
- Если очередь пуста, уменьшаете количество воркеров

```go
type ScalablePool struct {
    minWorkers int
    maxWorkers int
    current    int
    // другие поля
}

func (sp *ScalablePool) scale() {
    // логика масштабирования
}
```

### Задача 5: Priority Queue

Реализуйте Worker Pool с приоритетами:

- Задачи имеют приоритет (1-5)
- Высокоприоритетные задачи обрабатываются первыми
- Используйте несколько каналов или heap

```go
type PriorityJob struct {
    Priority int
    Data     interface{}
}

type PriorityPool struct {
    // реализация с приоритетами
}
```

## Решения Практических Задач

### Решение задачи 1: Базовый Pipeline

```go
package main

import (
    "context"
    "fmt"
)

func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func multiply(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * 2:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func merge(ctx context.Context, channels ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup

    multiplex := func(ch <-chan int) {
        defer wg.Done()
        for val := range ch {
            select {
            case out <- val:
            case <-ctx.Done():
                return
            }
        }
    }

    wg.Add(len(channels))
    for _, ch := range channels {
        go multiplex(ch)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}

func solution() int {
    ctx := context.Background()

    gen := generate(ctx, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

    c1 := multiply(ctx, gen)
    c2 := multiply(ctx, gen)
    c3 := multiply(ctx, gen)

    sum := 0
    for val := range merge(ctx, c1, c2, c3) {
        sum += val
    }

    return sum // 110 (сумма всех чисел * 2)
}
```

### Решение задачи 2: Rate Limiter

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type RateLimitedPool struct {
    workers            int
    requestsPerSecond  int
    jobs              chan Job
    results           chan Result
    wg                sync.WaitGroup
    ticker            *time.Ticker
}

func NewRateLimitedPool(workers, rps int) *RateLimitedPool {
    return &RateLimitedPool{
        workers:           workers,
        requestsPerSecond: rps,
        jobs:             make(chan Job, workers*2),
        results:          make(chan Result, workers*2),
        ticker:           time.NewTicker(time.Second / time.Duration(rps)),
    }
}

func (rlp *RateLimitedPool) worker(ctx context.Context, id int) {
    defer rlp.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-rlp.jobs:
            if !ok {
                return
            }

            // Ждем разрешения от rate limiter
            <-rlp.ticker.C

            // Обработка
            time.Sleep(time.Millisecond * 50)

            result := Result{
                Job:    job,
                Output: fmt.Sprintf("Processed by worker %d", id),
            }

            select {
            case rlp.results <- result:
            case <-ctx.Done():
                return
            }
        }
    }
}

func (rlp *RateLimitedPool) Start(ctx context.Context) {
    rlp.wg.Add(rlp.workers)
    for i := 0; i < rlp.workers; i++ {
        go rlp.worker(ctx, i)
    }
}

func (rlp *RateLimitedPool) Submit(job Job) {
    rlp.jobs <- job
}

func (rlp *RateLimitedPool) Shutdown() {
    close(rlp.jobs)
    rlp.wg.Wait()
    rlp.ticker.Stop()
    close(rlp.results)
}

func (rlp *RateLimitedPool) Results() <-chan Result {
    return rlp.results
}
```

### Решение задачи 3: Error Handling

```go
package main

import (
    "context"
    "fmt"
    "sync"
)

type Job struct {
    ID   int
    Data string
}

type Result struct {
    Job  Job
    Data interface{}
    Err  error
}

type ErrorHandlingPool struct {
    workers int
    jobs    chan Job
    results chan Result
    wg      sync.WaitGroup
}

func NewErrorHandlingPool(workers int) *ErrorHandlingPool {
    return &ErrorHandlingPool{
        workers: workers,
        jobs:    make(chan Job, workers),
        results: make(chan Result, workers),
    }
}

func (ehp *ErrorHandlingPool) worker(ctx context.Context, id int) {
    defer ehp.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-ehp.jobs:
            if !ok {
                return
            }

            // Обработка с возможной ошибкой
            data, err := processWithError(job)

            result := Result{
                Job:  job,
                Data: data,
                Err:  err,
            }

            select {
            case ehp.results <- result:
            case <-ctx.Done():
                return
            }
        }
    }
}

func processWithError(job Job) (interface{}, error) {
    // Имитация обработки с возможной ошибкой
    if job.ID == 5 {
        return nil, fmt.Errorf("error processing job %d", job.ID)
    }
    return fmt.Sprintf("Processed: %s", job.Data), nil
}

func (ehp *ErrorHandlingPool) StartWithErrorHandling(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    ehp.wg.Add(ehp.workers)
    for i := 0; i < ehp.workers; i++ {
        go ehp.worker(ctx, i)
    }

    // Горутина для закрытия results после завершения воркеров
    go func() {
        ehp.wg.Wait()
        close(ehp.results)
    }()

    // Мониторинг ошибок
    var firstErr error
    for result := range ehp.results {
        if result.Err != nil && firstErr == nil {
            firstErr = result.Err
            cancel() // Останавливаем все воркеры
        }
    }

    return firstErr
}

func (ehp *ErrorHandlingPool) Submit(job Job) {
    ehp.jobs <- job
}

func (ehp *ErrorHandlingPool) Close() {
    close(ehp.jobs)
}
```

## Заключение

Паттерны Fan-In и Fan-Out являются фундаментальными инструментами для построения эффективных конкурентных систем в Go. Понимание этих паттернов, правильное использование горутин, каналов, WaitGroup и Context позволяет создавать масштабируемые и надежные приложения.

Ключевые моменты для запоминания:

- Fan-Out распределяет работу между несколькими воркерами
- Fan-In собирает результаты из нескольких источников
- Всегда используйте Context для управления жизненным циклом
- WaitGroup для синхронизации завершения горутин
- Отправитель закрывает каналы, получатель проверяет закрытие
- Предотвращайте goroutine leaks через правильную отмену
- Используйте race detector для поиска проблем конкурентности
