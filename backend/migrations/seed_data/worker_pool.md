---
title: "Worker Pool"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

Worker Pool (пул воркеров) — это паттерн конкурентного программирования, при котором фиксированное количество горутин (workers) обрабатывает задачи из общей очереди. Это позволяет контролировать количество одновременно выполняющихся операций и эффективно использовать ресурсы.

## Зачем нужен Worker Pool

### Проблемы, которые решает Worker Pool

- **Контроль количества горутин** - Без ограничений можно создать тысячи горутин, что приведет к избыточному потреблению памяти и degradation производительности из-за context switching.

- **Управление ресурсами** - Позволяет ограничить количество одновременных подключений к базе данных, внешним API или другим ресурсам.

- **Graceful shutdown** - Упрощает корректное завершение работы приложения — можно дождаться завершения всех задач перед остановкой.

- **Переиспользование горутин** - Вместо создания и уничтожения горутин для каждой задачи, воркеры работают постоянно, что снижает overhead.

## Ключевые компоненты

### Каналы (Channels)

Каналы — это механизм коммуникации между горутинами. В Worker Pool используются:

- **Jobs channel** — канал для передачи задач воркерам
- **Results channel** — канал для получения результатов (опционально)

### WaitGroup

`sync.WaitGroup` используется для ожидания завершения всех воркеров. Основные методы:

- `Add(n)` — увеличивает счетчик на n
- `Done()` — уменьшает счетчик на 1
- `Wait()` — блокирует выполнение до обнуления счетчика

### Context

`context.Context` позволяет управлять жизненным циклом воркеров, передавать сигналы отмены и таймауты.

## Базовая реализация

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

// Job представляет задачу для обработки
type Job struct {
    ID   int
    Data string
}

// Result представляет результат обработки
type Result struct {
    JobID  int
    Output string
}

// Начните смотреть main сначала
// Worker обрабатывает задачи из канала jobs
func worker(id int, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for job := range jobs {
        fmt.Printf("Worker %d начал обработку job %d\n", id, job.ID)
        
        // Имитация работы
        time.Sleep(time.Second)
        
        result := Result{
            JobID:  job.ID,
            Output: fmt.Sprintf("Обработано воркером %d: %s", id, job.Data),
        }
        
        results <- result
    }
    
    fmt.Printf("Worker %d завершил работу\n", id)
}

func main() {
    const numWorkers = 3
    const numJobs = 10
    
    jobs := make(chan Job, numJobs)
    results := make(chan Result, numJobs)
    
    var wg sync.WaitGroup
    
    // Запускаем воркеры
    // Для них пока нет задач, поэтому они будут 
    // ждать на строчке for job := range
    for i := 1; i <= numWorkers; i++ {
        wg.Add(1)
        go worker(i, jobs, results, &wg)
    }
    
    // Отправляем задачи. Воркеры начинают работать.
    for j := 1; j <= numJobs; j++ {
        jobs <- Job{
            ID:   j,
            Data: fmt.Sprintf("данные_%d", j),
        }
    }
    close(jobs) // Закрываем канал после отправки всех задач
    
    // Ждем завершения всех воркеров в отдельной горутине
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Собираем результаты
    for result := range results {
        fmt.Printf("Результат job %d: %s\n", result.JobID, result.Output)
    }
}
```

## Продвинутые техники

### Worker Pool с контекстом

```go
func workerWithContext(ctx context.Context, id int, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d: получен сигнал отмены\n", id)
            return
        case job, ok := <-jobs:
            if !ok {
                return
            }
            
            // Обработка с проверкой контекста
            select {
            case <-ctx.Done():
                return
            default:
                // Выполняем работу
                result := processJob(job)
                results <- result
            }
        }
    }
}
```

### Динамический Worker Pool

```go
type WorkerPool struct {
    maxWorkers int
    jobs       chan Job
    results    chan Result
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func NewWorkerPool(maxWorkers int) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    return &WorkerPool{
        maxWorkers: maxWorkers,
        jobs:       make(chan Job, 100),
        results:    make(chan Result, 100),
        ctx:        ctx,
        cancel:     cancel,
    }
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.maxWorkers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }
}

func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    
    for {
        select {
        case <-wp.ctx.Done():
            return
        case job, ok := <-wp.jobs:
            if !ok {
                return
            }
            // Обработка задачи
            result := processJob(job)
            wp.results <- result
        }
    }
}

func (wp *WorkerPool) Submit(job Job) {
    wp.jobs <- job
}

func (wp *WorkerPool) Shutdown() {
    close(wp.jobs)
    wp.wg.Wait()
    close(wp.results)
}

func (wp *WorkerPool) Stop() {
    wp.cancel()
    wp.wg.Wait()
}
```

### Обработка ошибок

```go
type Result struct {
    JobID  int
    Output string
    Err    error
}

func worker(id int, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for job := range jobs {
        output, err := processJobWithError(job)
        
        results <- Result{
            JobID:  job.ID,
            Output: output,
            Err:    err,
        }
    }
}

// В main
for result := range results {
    if result.Err != nil {
        fmt.Printf("Ошибка в job %d: %v\n", result.JobID, result.Err)
        continue
    }
    fmt.Printf("Успех job %d: %s\n", result.JobID, result.Output)
}
```

## Важные концепции и термины

### Buffered vs Unbuffered Channels

**Unbuffered channel** — отправка блокируется до получения  
**Buffered channel** — отправка блокируется только когда буфер полон

```go
jobs := make(chan Job)        // unbuffered
jobs := make(chan Job, 100)   // buffered с capacity 100
```

### Закрытие каналов

Отправитель должен закрывать канал, а не получатель. Закрытие канала сигнализирует получателям, что данных больше не будет.

```go
close(jobs) // После этого чтение из канала вернет zero value и false
```

### Deadlock

Deadlock возникает когда горутины взаимно блокируют друг друга. В Worker Pool это может произойти если:

- Забыли закрыть канал jobs
- Ждем результаты, но канал results не закрыт
- WaitGroup неправильно используется

### Rate Limiting

Ограничение скорости обработки задач:

```go
ticker := time.NewTicker(time.Second / 10) // 10 запросов в секунду
defer ticker.Stop()

for job := range jobs {
    <-ticker.C // Ждем тик
    processJob(job)
}
```

### Semaphore Pattern

Альтернатива Worker Pool через буферизованный канал:

```go
sem := make(chan struct{}, maxConcurrent)

for _, job := range jobs {
    sem <- struct{}{} // Получаем "токен"
    go func(j Job) {
        defer func() { <-sem }() // Возвращаем "токен"
        processJob(j)
    }(job)
}

// Ждем освобождения всех токенов
for i := 0; i < cap(sem); i++ {
    sem <- struct{}{}
}
```

## Паттерны использования

### Fan-out, Fan-in

**Fan-out** — распределение работы между несколькими воркерами  
**Fan-in** — сбор результатов от нескольких воркеров в один канал

```go
func fanIn(channels ...<-chan Result) <-chan Result {
    var wg sync.WaitGroup
    multiplexed := make(chan Result)
    
    multiplex := func(c <-chan Result) {
        defer wg.Done()
        for result := range c {
            multiplexed <- result
        }
    }
    
    wg.Add(len(channels))
    for _, c := range channels {
        go multiplex(c)
    }
    
    go func() {
        wg.Wait()
        close(multiplexed)
    }()
    
    return multiplexed
}
```

### Pipeline Pattern

Цепочка обработки, где выход одного воркера — вход следующего:

```go
func stage1(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            out <- n * 2
        }
    }()
    return out
}

func stage2(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            out <- n + 1
        }
    }()
    return out
}

// Использование
c := generate(1, 2, 3)
c = stage1(c)
c = stage2(c)
```

## Производительность и оптимизация

### Выбор количества воркеров

- **CPU-bound задачи**: `runtime.NumCPU()` или `runtime.GOMAXPROCS(0)`
- **I/O-bound задачи**: больше воркеров (экспериментально)
- **Внешние ограничения**: по лимитам API, БД connections и т.д.

```go
numWorkers := runtime.NumCPU()
```

### Размер буфера канала

- Слишком маленький — частые блокировки
- Слишком большой — избыточное потребление памяти
- Рекомендация: начать с количества воркеров * 2-3

### Избежание allocation

Переиспользование объектов через `sync.Pool`:

```go
var jobPool = sync.Pool{
    New: func() interface{} {
        return &Job{}
    },
}

job := jobPool.Get().(*Job)
// Использование
jobPool.Put(job)
```

## Тестирование

### Юнит-тесты

```go
func TestWorkerPool(t *testing.T) {
    jobs := make(chan Job, 10)
    results := make(chan Result, 10)
    var wg sync.WaitGroup
    
    // Запуск воркера
    wg.Add(1)
    go worker(1, jobs, results, &wg)
    
    // Отправка тестовой задачи
    testJob := Job{ID: 1, Data: "test"}
    jobs <- testJob
    close(jobs)
    
    // Проверка результата
    result := <-results
    if result.JobID != testJob.ID {
        t.Errorf("Expected JobID %d, got %d", testJob.ID, result.JobID)
    }
    
    wg.Wait()
    close(results)
}
```

### Бенчмарки

```go
func BenchmarkWorkerPool(b *testing.B) {
    numWorkers := runtime.NumCPU()
    jobs := make(chan Job, 100)
    results := make(chan Result, 100)
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        var wg sync.WaitGroup
        
        for w := 0; w < numWorkers; w++ {
            wg.Add(1)
            go worker(w, jobs, results, &wg)
        }
        
        for j := 0; j < 100; j++ {
            jobs <- Job{ID: j, Data: "benchmark"}
        }
        close(jobs)
        
        go func() {
            wg.Wait()
            close(results)
        }()
        
        for range results {
        }
    }
}
```

## Частые ошибки

### Забыли закрыть канал

```go
// НЕПРАВИЛЬНО
for job := range jobs {
    processJob(job)
}
// Если jobs не закрыт, цикл будет ждать вечно

// ПРАВИЛЬНО
close(jobs) // В main после отправки всех задач
```

### Неправильное использование WaitGroup

```go
// НЕПРАВИЛЬНО
wg.Add(1)
go func() {
    wg.Done()
    // wg.Add(1) - добавление внутри горутины опасно
}()

// ПРАВИЛЬНО
wg.Add(1)
go func() {
    defer wg.Done()
    // работа
}()
```

### Гонка данных при доступе к shared state

```go
// НЕПРАВИЛЬНО
var counter int
for job := range jobs {
    counter++ // race condition
}

// ПРАВИЛЬНО
var mu sync.Mutex
for job := range jobs {
    mu.Lock()
    counter++
    mu.Unlock()
}

// ИЛИ используйте atomic
var counter int64
atomic.AddInt64(&counter, 1)
```

### Утечка горутин

```go
// НЕПРАВИЛЬНО - воркеры могут остаться запущенными
go worker(jobs, results)

// ПРАВИЛЬНО - используйте context для контроля
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
go workerWithContext(ctx, jobs, results)
```

## Частые вопросы на собеседованиях

### Базовые вопросы

**1. Что такое Worker Pool и зачем он нужен?**

Worker Pool — это паттерн, где фиксированное количество горутин обрабатывает задачи из общей очереди. Нужен для контроля количества горутин, управления ресурсами и graceful shutdown.

**2. В чем разница между запуском N горутин для N задач и использованием Worker Pool?**

При запуске N горутин: неконтролируемое потребление ресурсов, сложность управления. Worker Pool: фиксированное количество воркеров, контролируемое использование ресурсов, легче управлять жизненным циклом.

**3. Как определить оптимальное количество воркеров?**

Зависит от типа задач: для CPU-bound — `runtime.NumCPU()`, для I/O-bound — больше (экспериментально), с учетом внешних ограничений (лимиты API, connection pool БД).

**4. Что произойдет если не закрыть канал jobs?**

Воркеры будут ждать новых задач бесконечно, `range` не завершится, WaitGroup никогда не обнулится — deadlock или зависание программы.

**5. Зачем нужен буфер в канале?**

Буфер позволяет отправителю не блокироваться пока получатель не готов. Уменьшает latency, но увеличивает потребление памяти.

### Продвинутые вопросы

**6. Как реализовать graceful shutdown Worker Pool?**

Использовать context для сигнала отмены, закрыть канал jobs, дождаться завершения через WaitGroup, затем закрыть results.

**7. Как обрабатывать ошибки в Worker Pool?**

Включить поле Err в структуру Result, воркеры возвращают ошибки через results channel, main обрабатывает их при чтении результатов.

**8. В чем разница между close(channel) и channel = nil?**

`close(channel)` — сигнализирует о завершении отправки, чтение возвращает zero value и false. `channel = nil` — просто обнуляет переменную, не влияет на сам канал.

**9. Что такое goroutine leak и как его избежать в Worker Pool?**

Утечка горутин — когда горутины остаются запущенными после завершения работы. Избежать: использовать context с timeout/cancel, правильно закрывать каналы, использовать WaitGroup.

**10. Как реализовать rate limiting в Worker Pool?**

Использовать `time.Ticker` для ограничения частоты обработки или библиотеку `golang.org/x/time/rate` с `rate.Limiter`.

**11. Объясните паттерн Fan-out/Fan-in**

Fan-out — распределение задач между несколькими воркерами (один канал -> много воркеров). Fan-in — сбор результатов от нескольких воркеров (много каналов -> один канал результатов).

**12. Когда использовать Worker Pool вместо semaphore pattern?**

Worker Pool: нужен контроль над жизненным циклом воркеров, переиспользование ресурсов, сложная логика обработки. Semaphore: простое ограничение количества одновременных операций, короткоживущие задачи.

## Практические задания

### Задание 1: Базовый Worker Pool

Реализуйте Worker Pool для скачивания файлов по URL:

```go
type DownloadJob struct {
    URL      string
    Filename string
}

type DownloadResult struct {
    Filename string
    Size     int64
    Err      error
}

// Реализуйте:
// 1. Worker функцию для скачивания
// 2. Main функцию с запуском 5 воркеров
// 3. Обработку ошибок
// 4. Вывод статистики
```

### Задание 2: Worker Pool с timeout

Модифицируйте базовый Worker Pool:

- Добавьте timeout 5 секунд для каждой задачи
- Если задача не завершилась за 5 секунд, отменяйте ее
- Логируйте отмененные задачи
- Используйте `context.WithTimeout`

### Задание 3: Динамический Worker Pool

Создайте Worker Pool с возможностью:

- Добавления новых задач после старта
- Изменения количества воркеров во время работы
- Получения метрик (количество обработанных задач, среднее время обработки)
- Graceful shutdown

### Задание 4: Pipeline с Worker Pools

Реализуйте pipeline для обработки изображений:

1. Stage 1: Чтение файлов (3 воркера)
2. Stage 2: Изменение размера (5 воркеров)
3. Stage 3: Применение фильтров (5 воркеров)
4. Stage 4: Сохранение результата (2 воркера)

Каждый stage — отдельный Worker Pool.

### Задание 5: Rate Limited Worker Pool

Реализуйте Worker Pool для вызовов API с ограничениями:

- Максимум 10 запросов в секунду
- Максимум 100 запросов в минуту
- При превышении лимита — exponential backoff
- Обработка ошибок 429 (Too Many Requests)

### Пример решения задания 1

```go
package main

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "sync"
    "time"
)

type DownloadJob struct {
    URL      string
    Filename string
}

type DownloadResult struct {
    Filename string
    Size     int64
    Duration time.Duration
    Err      error
}

func downloadWorker(id int, jobs <-chan DownloadJob, results chan<- DownloadResult, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for job := range jobs {
        start := time.Now()
        
        fmt.Printf("Worker %d: начинаю загрузку %s\n", id, job.URL)
        
        // Скачивание файла
        resp, err := http.Get(job.URL)
        if err != nil {
            results <- DownloadResult{
                Filename: job.Filename,
                Err:      fmt.Errorf("ошибка GET: %w", err),
                Duration: time.Since(start),
            }
            continue
        }
        
        if resp.StatusCode != http.StatusOK {
            resp.Body.Close()
            results <- DownloadResult{
                Filename: job.Filename,
                Err:      fmt.Errorf("статус %d", resp.StatusCode),
                Duration: time.Since(start),
            }
            continue
        }
        
        // Создание файла
        file, err := os.Create(job.Filename)
        if err != nil {
            resp.Body.Close()
            results <- DownloadResult{
                Filename: job.Filename,
                Err:      fmt.Errorf("ошибка создания файла: %w", err),
                Duration: time.Since(start),
            }
            continue
        }
        
        // Копирование данных
        size, err := io.Copy(file, resp.Body)
        file.Close()
        resp.Body.Close()
        
        if err != nil {
            results <- DownloadResult{
                Filename: job.Filename,
                Size:     size,
                Err:      fmt.Errorf("ошибка копирования: %w", err),
                Duration: time.Since(start),
            }
            continue
        }
        
        results <- DownloadResult{
            Filename: job.Filename,
            Size:     size,
            Duration: time.Since(start),
        }
    }
}

func main() {
    urls := []string{
        "https://example.com/file1.txt",
        "https://example.com/file2.txt",
        "https://example.com/file3.txt",
        "https://example.com/file4.txt",
        "https://example.com/file5.txt",
    }
    
    numWorkers := 5
    jobs := make(chan DownloadJob, len(urls))
    results := make(chan DownloadResult, len(urls))
    
    var wg sync.WaitGroup
    
    // Запуск воркеров
    for i := 1; i <= numWorkers; i++ {
        wg.Add(1)
        go downloadWorker(i, jobs, results, &wg)
    }
    
    // Отправка задач
    for i, url := range urls {
        jobs <- DownloadJob{
            URL:      url,
            Filename: fmt.Sprintf("file_%d.txt", i+1),
        }
    }
    close(jobs)
    
    // Ожидание завершения
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Статистика
    var totalSize int64
    var successCount, errorCount int
    var totalDuration time.Duration
    
    for result := range results {
        if result.Err != nil {
            fmt.Printf("❌ %s: %v (за %v)\n", result.Filename, result.Err, result.Duration)
            errorCount++
        } else {
            fmt.Printf("✅ %s: %d байт (за %v)\n", result.Filename, result.Size, result.Duration)
            totalSize += result.Size
            successCount++
        }
        totalDuration += result.Duration
    }
    
    fmt.Println("\n=== Статистика ===")
    fmt.Printf("Успешно: %d\n", successCount)
    fmt.Printf("Ошибок: %d\n", errorCount)
    fmt.Printf("Всего загружено: %d байт\n", totalSize)
    fmt.Printf("Среднее время: %v\n", totalDuration/time.Duration(successCount+errorCount))
}
```

## Полезные ссылки

- Go Concurrency Patterns (официальный блог Go)
- Effective Go — раздел Concurrency
- `golang.org/x/sync` — расширенные примитивы синхронизации
- `golang.org/x/time/rate` — rate limiting

## Заключение

Worker Pool — фундаментальный паттерн в Go для управления конкурентностью. Понимание принципов работы каналов, WaitGroup, context и правильная обработка ошибок — ключ к созданию надежных и производительных приложений.

Основные принципы:

- Контролируйте количество горутин
- Всегда закрывайте каналы в правильном месте
- Используйте context для управления жизненным циклом
- Обрабатывайте ошибки явно
- Тестируйте и бенчмаркайте ваши решения
