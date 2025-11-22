---
title: "Семафор"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

Семафор — это примитив синхронизации, который контролирует доступ к общему ресурсу путём **ограничения количества одновременных операций**. В отличие от мьютекса, который разрешает доступ только одной горутине, семафор может разрешить доступ **определённому числу горутин одновременно**.

В Go семафор используется для ограничения конкурентного доступа и реализации паттерна worker pool — пула воркеров, где количество одновременно работающих горутин строго ограничено.

## Реализации семафора в Go

### Буферизованный канал

Самый идиоматичный способ реализации семафора в Go — использование буферизованного канала (buffered channel). Размер буфера канала определяет максимальное количество одновременных операций.

```go
// Создание семафора на 5 слотов
sem := make(chan struct{}, 5)

// Захват семафора (acquire)
// Как бы увеличили счетчик на один
sem <- struct{}{}

// Освобождение семафора (release)
// Уменьшили счетчик на один
<-sem
```

Принцип работы:

- Отправка в канал блокируется, когда буфер заполнен — это acquire операция
- Чтение из канала освобождает место в буфере — это release операция
- Используется пустая структура `struct{}{}`, так как она не занимает память

Пример worker pool с буферизованным каналом:

```go
func processFiles(files []string) {
    maxWorkers := 5
    sem := make(chan struct{}, maxWorkers)

    for _, file := range files {
        sem <- struct{}{} // acquire

        go func(f string) {
            defer func() { <-sem }() // release
            processFile(f)
        }(file)
    }

    // Ждём завершения всех воркеров
    for i := 0; i < maxWorkers; i++ {
        sem <- struct{}{}
        // Пройдем весь цикл только когда все
        // места станут свободными - тогда 
        // перейдем на след строчку. Похоже на
        // wg.Wait()
    }
}
```

### Пакет golang.org/x/sync/semaphore

Go предоставляет официальную реализацию взвешенного семафора (weighted semaphore) в extended sync package. Взвешенный семафор позволяет разным задачам занимать разное количество слотов.

В отличие от простой реализации тут просто важно знать что такое есть.

```go
import "golang.org/x/sync/semaphore"

// Создание взвешенного семафора
sem := semaphore.NewWeighted(10)

// Захват с весом 1
ctx := context.Background()
if err := sem.Acquire(ctx, 1); err != nil {
    log.Fatal(err)
}

// Освобождение
defer sem.Release(1)
```

Ключевые методы:

- `NewWeighted(n int64)` — создаёт семафор с максимальным весом n
- `Acquire(ctx context.Context, n int64)` — захватывает n единиц, блокируется до освобождения ресурсов или отмены контекста
- `Release(n int64)` — освобождает n единиц
- `TryAcquire(n int64)` — неблокирующая попытка захвата

## Контекст и семафор

Использование контекста (context) с семафором обеспечивает graceful shutdown и контроль времени выполнения.

```go
func worker(ctx context.Context, sem *semaphore.Weighted) error {
    // Acquire с таймаутом
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    if err := sem.Acquire(ctx, 1); err != nil {
        return fmt.Errorf("failed to acquire: %w", err)
    }
    defer sem.Release(1)

    // Работа с возможностью отмены
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // выполнение задачи
    }

    return nil
}
```

Важные практики:

- Всегда передавайте контекст в `Acquire` для возможности отмены
- Используйте `defer sem.Release()` сразу после успешного `Acquire`
- Проверяйте `ctx.Done()` в длительных операциях для graceful cancellation

## Взвешенный семафор на практике

Взвешенный семафор полезен, когда разные задачи требуют разного количества ресурсов.

```go
func processMedia(files []File, sem *semaphore.Weighted) {
    for _, file := range files {
        var weight int64

        switch file.Type {
        case "image":
            weight = 1 // лёгкая задача
        case "video":
            weight = 3 // тяжёлая задача
        case "pdf":
            weight = 2 // средняя задача
        }

        go func(f File, w int64) {
            if err := sem.Acquire(context.Background(), w); err != nil {
                return
            }
            defer sem.Release(w)

            processFile(f)
        }(file, weight)
    }
}
```

При этом если семафор создан с весом 5, то одновременно могут выполняться:

- 5 задач с весом 1
- 1 задача с весом 3 и 2 задачи с весом 1
- 2 задачи с весом 2 и 1 задача с весом 1

## Семафор vs другие примитивы

### Семафор vs Mutex

- Mutex — бинарный семафор (только 0 или 1), разрешает доступ только одной горутине
- Семафор может разрешить доступ N горутинам одновременно
- Mutex используется для критических секций, семафор — для ограничения конкурентности

### Семафор vs WaitGroup

- WaitGroup — это counting semaphore для ожидания завершения группы горутин
- WaitGroup не ограничивает количество одновременно работающих горутин
- Семафор ограничивает конкурентность, WaitGroup только отслеживает завершение

### Семафор vs Buffered Channel

- Буферизованный канал — это простейшая реализация семафора
- `golang.org/x/sync/semaphore` предоставляет weighted семафор и интеграцию с контекстом
- Для простых случаев достаточно буферизованного канала

## Типичные ошибки и best practices

### Ошибка: забыть Release

```go
// Плохо
sem.Acquire(ctx, 1)
doWork()
sem.Release(1) // если doWork() запаникует, Release не вызовется
```

```go
// Хорошо
if err := sem.Acquire(ctx, 1); err != nil {
    return err
}
defer sem.Release(1)
doWork()
```

### Ошибка: неправильный размер семафора

```go
// Плохо — слишком много воркеров
sem := semaphore.NewWeighted(1000) // может исчерпать ресурсы
```

```go
// Хорошо — ограничено числом CPU
maxWorkers := runtime.GOMAXPROCS(0)
sem := semaphore.NewWeighted(int64(maxWorkers))
```

### Ошибка: игнорирование контекста

```go
// Плохо — нет возможности отменить
sem.Acquire(context.Background(), 1)
```

```go
// Хорошо — с таймаутом или отменой
ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
defer cancel()
if err := sem.Acquire(ctx, 1); err != nil {
    return err
}
```

### Best practices

- Всегда используйте `defer` для Release после успешного Acquire
- Передавайте контекст для возможности cancellation и timeout
- Выбирайте размер семафора на основе доступных ресурсов (CPU, memory, network connections)
- Для простых случаев используйте буферизованный канал, для сложных — `golang.org/x/sync/semaphore`
- Используйте `TryAcquire` для неблокирующих операций
- При использовании weighted semaphore следите, чтобы ни одна задача не запрашивала вес больше максимального

## Паттерн Worker Pool

Worker pool — классический паттерн использования семафора для ограничения количества одновременно работающих горутин.

```go
func workerPool(tasks []Task, maxWorkers int) {
    sem := semaphore.NewWeighted(int64(maxWorkers))
    ctx := context.Background()

    for _, task := range tasks {
        if err := sem.Acquire(ctx, 1); err != nil {
            log.Printf("Failed to acquire: %v", err)
            break
        }

        go func(t Task) {
            defer sem.Release(1)
            processTask(t)
        }(task)
    }

    // Ждём завершения всех воркеров
    if err := sem.Acquire(ctx, int64(maxWorkers)); err != nil {
        log.Printf("Failed to wait: %v", err)
    }
}
```

Альтернатива с WaitGroup для синхронизации:

```go
func workerPoolWithWaitGroup(tasks []Task, maxWorkers int) {
    sem := semaphore.NewWeighted(int64(maxWorkers))
    var wg sync.WaitGroup

    for _, task := range tasks {
        wg.Add(1)

        go func(t Task) {
            defer wg.Done()

            if err := sem.Acquire(context.Background(), 1); err != nil {
                return
            }
            defer sem.Release(1)

            processTask(t)
        }(task)
    }

    wg.Wait()
}
```

## Частые вопросы на собеседованиях

### Теоретические вопросы

1. **Что такое семафор и чем он отличается от мутекса?**
   - Ожидаемый ответ: семафор позволяет N горутинам одновременный доступ, мутекс — только одной. Семафор используется для ограничения конкурентности, мутекс для защиты критических секций.

2. **Как реализовать семафор в Go?**
   - Ожидаемый ответ: через буферизованный канал или пакет `golang.org/x/sync/semaphore`. Описать принцип работы с буферизованным каналом.

3. **Что такое weighted semaphore и когда его использовать?**
   - Ожидаемый ответ: семафор, где разные задачи могут занимать разное количество слотов. Используется когда задачи требуют разных объёмов ресурсов.

4. **Зачем использовать context с семафором?**
   - Ожидаемый ответ: для graceful shutdown, timeout, cancellation. Позволяет прервать ожидание захвата семафора.

5. **В чём разница между WaitGroup и семафором?**
   - Ожидаемый ответ: WaitGroup ждёт завершения горутин, но не ограничивает их количество. Семафор ограничивает количество одновременно работающих горутин.

6. **Что произойдёт если забыть вызвать Release?**
   - Ожидаемый ответ: семафор останется в состоянии занятости, уменьшится количество доступных слотов. Может привести к deadlock или снижению throughput.

7. **Можно ли изменить размер семафора динамически?**
   - Ожидаемый ответ: стандартный пакет `golang.org/x/sync/semaphore` не поддерживает изменение размера. Нужна кастомная реализация или пересоздание семафора.

### Практические вопросы

1. **Реализуйте rate limiter на основе семафора**

```go
type RateLimiter struct {
    sem      *semaphore.Weighted
    rate     int
    interval time.Duration
}

func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
    rl := &RateLimiter{
        sem:      semaphore.NewWeighted(int64(rate)),
        rate:     rate,
        interval: interval,
    }

    go rl.refill()
    return rl
}

func (rl *RateLimiter) refill() {
    ticker := time.NewTicker(rl.interval)
    defer ticker.Stop()

    for range ticker.C {
        // Освобождаем все слоты
        for i := 0; i < rl.rate; i++ {
            if rl.sem.TryAcquire(1) {
                rl.sem.Release(1)
            }
        }
    }
}

func (rl *RateLimiter) Allow(ctx context.Context) error {
    return rl.sem.Acquire(ctx, 1)
}
```

2. **Напишите функцию параллельной обработки с ограничением**

```go
func ProcessConcurrently(items []string, maxConcurrent int, 
    process func(string) error) error {

    sem := semaphore.NewWeighted(int64(maxConcurrent))
    ctx := context.Background()

    var wg sync.WaitGroup
    errChan := make(chan error, len(items))

    for _, item := range items {
        wg.Add(1)

        go func(i string) {
            defer wg.Done()

            if err := sem.Acquire(ctx, 1); err != nil {
                errChan <- err
                return
            }
            defer sem.Release(1)

            if err := process(i); err != nil {
                errChan <- err
            }
        }(item)
    }

    wg.Wait()
    close(errChan)

    for err := range errChan {
        if err != nil {
            return err
        }
    }

    return nil
}
```

3. **Как предотвратить goroutine leak при использовании семафора?**

```go
func SafeWorkerPool(ctx context.Context, tasks []Task, maxWorkers int) error {
    sem := semaphore.NewWeighted(int64(maxWorkers))
    var wg sync.WaitGroup

    for _, task := range tasks {
        // Проверяем контекст перед запуском
        select {
        case <-ctx.Done():
            wg.Wait() // ждём завершения запущенных
            return ctx.Err()
        default:
        }

        wg.Add(1)
        go func(t Task) {
            defer wg.Done()

            if err := sem.Acquire(ctx, 1); err != nil {
                return // горутина завершится при отмене контекста
            }
            defer sem.Release(1)

            // Работа с проверкой контекста
            select {
            case <-ctx.Done():
                return
            default:
                processTask(t)
            }
        }(task)
    }

    wg.Wait()
    return nil
}
```

## Практические задания

### Задание 1: Базовый worker pool

Реализуйте worker pool для обработки списка URL. Ограничьте количество одновременных HTTP запросов до 10.

Решение

```go
func FetchURLs(urls []string) ([]Response, error) {
    maxConcurrent := 10
    sem := semaphore.NewWeighted(int64(maxConcurrent))
    ctx := context.Background()

    results := make([]Response, len(urls))
    var wg sync.WaitGroup

    for i, url := range urls {
        wg.Add(1)

        go func(idx int, u string) {
            defer wg.Done()

            if err := sem.Acquire(ctx, 1); err != nil {
                return
            }
            defer sem.Release(1)

            resp, err := http.Get(u)
            if err != nil {
                results[idx] = Response{Error: err}
                return
            }
            defer resp.Body.Close()

            body, _ := io.ReadAll(resp.Body)
            results[idx] = Response{Body: body, Status: resp.StatusCode}
        }(i, url)
    }

    wg.Wait()
    return results, nil
}
```

### Задание 2: Weighted семафор для обработки файлов

Создайте обработчик файлов, где маленькие файлы (<1MB) занимают вес 1, средние (1-10MB) вес 2, большие (>10MB) вес 3. Общий вес семафора — 10.

Решение

```go
func ProcessFiles(files []File) error {
    sem := semaphore.NewWeighted(10)
    ctx := context.Background()
    var wg sync.WaitGroup

    for _, file := range files {
        weight := calculateWeight(file.Size)

        wg.Add(1)
        go func(f File, w int64) {
            defer wg.Done()

            if err := sem.Acquire(ctx, w); err != nil {
                log.Printf("Failed to acquire: %v", err)
                return
            }
            defer sem.Release(w)

            if err := processFile(f); err != nil {
                log.Printf("Failed to process %s: %v", f.Name, err)
            }
        }(file, weight)
    }

    wg.Wait()
    return nil
}

func calculateWeight(size int64) int64 {
    const MB = 1024 * 1024

    switch {
    case size < MB:
        return 1
    case size < 10*MB:
        return 2
    default:
        return 3
    }
}
```

### Задание 3: Семафор с graceful shutdown

Реализуйте worker pool с возможностью graceful shutdown по сигналу (SIGINT/SIGTERM).

Решение

```go
func WorkerPoolWithShutdown(tasks <-chan Task, maxWorkers int) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Обработка сигналов
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutdown signal received")
        cancel()
    }()

    sem := semaphore.NewWeighted(int64(maxWorkers))
    var wg sync.WaitGroup

    for task := range tasks {
        select {
        case <-ctx.Done():
            log.Println("Stopping new tasks")
            wg.Wait()
            return
        default:
        }

        wg.Add(1)
        go func(t Task) {
            defer wg.Done()

            if err := sem.Acquire(ctx, 1); err != nil {
                return
            }
            defer sem.Release(1)

            processTaskWithContext(ctx, t)
        }(task)
    }

    wg.Wait()
}

func processTaskWithContext(ctx context.Context, task Task) {
    done := make(chan struct{})

    go func() {
        defer close(done)
        task.Execute()
    }()

    select {
    case <-done:
        log.Println("Task completed")
    case <-ctx.Done():
        log.Println("Task cancelled")
    }
}
```

## Резюме

Семафор в Go — мощный инструмент для контроля конкурентности. Ключевые моменты:

- Используйте буферизованный канал для простых случаев
- Применяйте `golang.org/x/sync/semaphore` для weighted семафора и интеграции с контекстом
- Всегда используйте `defer Release()` после `Acquire()`
- Передавайте контекст для graceful cancellation
- Выбирайте размер семафора на основе доступных ресурсов
- Комбинируйте с WaitGroup для надёжной синхронизации
