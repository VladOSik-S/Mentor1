---
title: "WaitGroup"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

WaitGroup — это примитив синхронизации из пакета `sync`, который позволяет дождаться завершения группы горутин. Это счетчик-семафор, который блокирует выполнение программы до тех пор, пока все запущенные горутины не завершат свою работу.

> WaitGroup решает проблему координации конкурентного выполнения: когда вы запускаете несколько горутин и вам нужно убедиться, что все они завершились перед продолжением работы основной программы.

## Основные методы

WaitGroup предоставляет три ключевых метода:

### Add(delta int)

Метод `Add` изменяет внутренний счетчик WaitGroup на значение `delta`. Положительное значение увеличивает счетчик (добавляет задачи), отрицательное уменьшает. Вызов `Add` должен происходить до запуска горутины, а не внутри неё.

### Done()

Метод `Done` уменьшает счетчик на единицу. Это сокращённая форма `Add(-1)`. Вызывается внутри горутины после завершения работы. Обычно используется с `defer`, чтобы гарантировать выполнение даже при панике.

### Wait()

Метод `Wait` блокирует выполнение вызывающей горутины до тех пор, пока счетчик не станет равным нулю. После этого программа продолжает выполнение.

## Базовый пример использования

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

func worker(id int, wg *sync.WaitGroup) {
    defer wg.Done()
    fmt.Printf("Worker %d starting\n", id)
    time.Sleep(time.Second)
    fmt.Printf("Worker %d done\n", id)
}

func main() {
    var wg sync.WaitGroup
    for i := 1; i <= 5; i++ {
        wg.Add(1)
        go worker(i, &wg)
    }
    wg.Wait()
    fmt.Println("All workers completed")
}
```

## Важные концепции и термины

**Передача по указателю** - WaitGroup всегда передается в функции по указателю (`*sync.WaitGroup`). Если передать по значению, каждая горутина получит свою копию счетчика — это бессмысленно для синхронизации.

**Happens-before гарантии** - Вызов `Add` должен происходить до вызова `Wait` в том же цикле синхронизации.

**Zero value** - WaitGroup можно использовать сразу после объявления без явной инициализации:

```go
var wg sync.WaitGroup
```

## Типичные ошибки и как их избежать

### Ошибка 1: Add внутри горутины

```go
// НЕПРАВИЛЬНО
var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    go func() {
        wg.Add(1)
        defer wg.Done()
        // работа
    }()
}
wg.Wait()
```

**Проблема:** Добавление задач внутри горутины может привести к ситуации, когда main уже вызовет `Wait` и программа завершится преждевременно.

```go
// ПРАВИЛЬНО
var wg sync.WaitGroup
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        // работа
    }()
}
wg.Wait()
```

### Ошибка 2: Передача по значению

```go
// НЕПРАВИЛЬНО
func process(wg sync.WaitGroup) {
    defer wg.Done()
}

// ПРАВИЛЬНО
func process(wg *sync.WaitGroup) {
    defer wg.Done()
}
```

### Ошибка 3: Отрицательный счетчик

```go
var wg sync.WaitGroup
wg.Add(2)
wg.Done()
wg.Done()
wg.Done() // panic: sync: negative WaitGroup counter
```

Вызывайте `Done` только столько раз, сколько было вызвано `Add(1)`.

### Ошибка 4: Повторное использование до завершения Wait

```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    time.Sleep(100 * time.Millisecond)
    wg.Done()
}()

// В другой горутине
go func() {
    wg.Wait()
    wg.Add(1) // Может вызвать panic, если Wait ещё не вернулся везде
}()
```

Перед повторным использованием WaitGroup убедитесь, что все вызовы `Wait` завершились.

### Ошибка 5: Забытый defer Done

```go
func worker(wg *sync.WaitGroup) {
    // Если здесь произойдёт паника, Done не вызовется
    doWork()
    wg.Done()
}

// Правильно с defer
func worker(wg *sync.WaitGroup) {
    defer wg.Done()
    doWork()
}
```

## Продвинутые паттерны

### Паттерн с пулом воркеров

```go
func processItems(items []string) {
    var wg sync.WaitGroup
    workerCount := 3
    itemChan := make(chan string, len(items))

    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range itemChan {
                process(item)
            }
        }()
    }

    for _, item := range items {
        itemChan <- item
    }
    close(itemChan)
    wg.Wait()
}
```

### Динамическое добавление задач

```go
func dynamicWorkers() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            if needsMoreWork(id) {
                wg.Add(1)
                go additionalWork(&wg, id)
            }
        }(i)
    }
    wg.Wait()
}
```

## WaitGroup vs Channels

WaitGroup используется, когда нужно дождаться завершения горутин, и не нужно получать их результаты. Для сбора результатов или обработки ошибок используют каналы или errgroup.

## Практические задачи

### Задача: Параллельная обработка файлов

```go
func processFiles(files []string) {
    var wg sync.WaitGroup
    for _, file := range files {
        wg.Add(1)
        go func(filename string) {
            defer wg.Done()
            data, err := os.ReadFile(filename)
            if err != nil {
                log.Printf("Error reading %s: %v", filename, err)
                return
            }
            processData(data)
        }(file)
    }
    wg.Wait()
}
```

### Задача: Ограниченный параллелизм

```go
func processWithLimit(tasks []Task, maxWorkers int) {
    var wg sync.WaitGroup
    taskChan := make(chan Task, len(tasks))

    for i := 0; i < maxWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for task := range taskChan {
                task.Execute()
            }
        }()
    }

    for _, task := range tasks {
        taskChan <- task
    }
    close(taskChan)
    wg.Wait()
}
```

### Задача: Найдите ошибку

```go
func buggyCode() {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        go func() {
            wg.Add(1)
            defer wg.Done()
            doWork(i)
        }()
    }
    wg.Wait()
}
```

**Ошибки:**

1. wg.Add(1) вызывается внутри горутины (race condition).
2. Переменная i захватывается по ссылке.

**Исправленный вариант:**

```go
func fixedCode() {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            doWork(id)
        }(i)
    }
    wg.Wait()
}
```

### Задача: Реализуйте функцию MapParallel

```go
func MapParallel[T any, R any](items []T, fn func(T) R) []R {
    var wg sync.WaitGroup
    results := make([]R, len(items))

    for i, item := range items {
        wg.Add(1)
        go func(index int, value T) {
            defer wg.Done()
            results[index] = fn(value)
        }(i, item)
    }
    wg.Wait()
    return results
}
```

## Альтернативы: errgroup и каналы

Используйте `errgroup` для сбора ошибок или отмены через context:

```go
import "golang.org/x/sync/errgroup"

func processWithErrors(ctx context.Context, items []string) error {
    g, ctx := errgroup.WithContext(ctx)
    for _, item := range items {
        item := item
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }
    return g.Wait()
}
```

Сбор результатов с помощью каналов:

```go
func collectResults(tasks []Task) []Result {
    resultChan := make(chan Result, len(tasks))
    for _, task := range tasks {
        go func(t Task) {
            resultChan <- t.Execute()
        }(task)
    }
    results := make([]Result, 0, len(tasks))
    for i := 0; i < len(tasks); i++ {
        results = append(results, <-resultChan)
    }
    return results
}
```

## Производительность и оптимизация

WaitGroup быстрый и минимально нагружает систему. В Go 1.25+ доступен метод `WaitGroup.Go`. Не создавайте WaitGroup в горячих циклах — используйте один экземпляр для партии задач.

## Заключение

WaitGroup — простой и мощный инструмент для синхронизации горутин в Go. Главные правила:

- Всегда вызывайте `Add` до запуска горутины.
- Передавайте WaitGroup по указателю.
- Используйте `defer wg.Done()` для надёжности.
- Следите за балансом Add и Done.
- Используйте race detector для поиска проблем.

Понимание WaitGroup необходимо каждому Go-разработчику, работающему с конкурентностью.
