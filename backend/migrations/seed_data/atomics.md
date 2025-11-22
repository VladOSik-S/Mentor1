---
title: "Атомики"
tags:
  - многопоточка
is_public: true 
# preview: "..."
---

Атомики (atomic operations) — это низкоуровневые примитивы синхронизации для безопасной работы с данными, разделяемыми между несколькими горутинами, без явного использования блокировок (мьютексов). Атомические операции гарантируют, что **операции над памятью выполняются за один неделимый шаг на уровне CPU, что исключает состояния гонки (data races)**.

### Ключевая философия Go

В документации Go говорится: "Share memory by communicating; don't communicate by sharing memory". Значит примерно это: используйте каналы. Однако атомики позволяют эффективно работать с общей памятью в специфичных сценариях, когда это целесообразно.

## Где находятся атомики

Все функции для работы с атомиками находятся в пакете `sync/atomic` стандартной библиотеки Go. Импортировать его нужно так:

```go
import "sync/atomic"
```

## Главные концепции

### Atomicity

Атомарная операция выполняется целиком за один шаг. Это означает, что другая горутина не сможет увидеть состояние "между" операциями. Например, при атомарном чтении значения вы получите либо старое значение, либо новое — никогда не получите "полуписанное" значение.

### Гарантии упорядочения памяти (Memory Ordering)

Это не супер важно, но. Go предоставляет гарантии, называемые **Sequential Consistency** (последовательная консистентность). Это означает:

- Все атомарные операции, выполненные в программе, ведут себя так, как если бы они были выполнены в некотором последовательном порядке
- Если операция A "синхронизируется перед" операцией B (A happens-before B), то эффект A наблюдается перед B

## Основные функции и типы

### 1. Load и Store (чтение и запись)

Эти операции безопасно читают и записывают значения без мьютексов.

> Обратите внимание, что используется не int, а int32. int32 - оптимальный баланс между размером и производительностью, поэтому выбрали его.

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    var counter int32

    // Store — безопасно записываем значение
    atomic.StoreInt32(&counter, 10)

    // Load — безопасно читаем значение
    value := atomic.LoadInt32(&counter)
    fmt.Println(value) // 10
}
```

**Когда использовать:** Когда нужно безопасно прочитать или изменить примитивное значение из нескольких горутин.

### 2. Add (увеличение/уменьшение)

Атомически добавляет дельта-значение и возвращает новое значение. Идеально подходит для счетчиков.

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

func main() {
    var counter int32
    var wg sync.WaitGroup

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            atomic.AddInt32(&counter, 1)
            wg.Done()
        }()
    }

    wg.Wait()
    fmt.Println(atomic.LoadInt32(&counter)) // Гарантированно 1000
}
```

**Когда использовать:** Для конкурентных счетчиков, счета обработанных событий, меток времени.

### 3. Swap (обмен)

Атомически заменяет старое значение новым и возвращает старое значение.

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    var value int32 = 100

    oldValue := atomic.SwapInt32(&value, 200)

    fmt.Println("Новое:", value)     // 200
    fmt.Println("Старое:", oldValue) // 100
}
```

**Когда использовать:** Когда нужно одновременно заменить значение и получить предыдущее.

### 4. CompareAndSwap (CAS) — сравнение и обмен

Это **фундаментальная операция** для lock-free алгоритмов. Операция выполняется в один атомический шаг:

- Проверяется, что текущее значение равно старому значению
- **Если равно, заменяется на новое** и возвращает `true`
- **Если не равно, ничего не меняет** и возвращает `false`

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    var value int32 = 100

    // Попытка заменить 100 на 200
    swapped := atomic.CompareAndSwapInt32(&value, 100, 200)
    fmt.Println("Успешно:", swapped) // true
    fmt.Println("Значение:", value)  // 200

    // Попытка заменить 100 на 300 (100 больше не там!)
    swapped = atomic.CompareAndSwapInt32(&value, 100, 300)
    fmt.Println("Успешно:", swapped) // false
    fmt.Println("Значение:", value)  // 200 (не изменилось)
}
```

**Когда использовать:** Для оптимистичных блокировок, retry-логики, реализации lock-free структур данных.

### 5. And и Or (побитовые операции) — добавлены в Go 1.23

Атомически выполняют побитовые операции AND и OR с маской.

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    var flags int32 = 0b1111

    // Побитовое И
    old := atomic.AndInt32(&flags, 0b1010) // Оставляем только биты 1 и 3
    fmt.Println(old)   // 15 (0b1111)
    fmt.Println(flags) // 10 (0b1010)
}
```

## Современные типы (Go 1.19+)

С версии Go 1.19 введены типизированные обертки над функциями. Они более удобны и безопаснее, так как работают с типами вместо указателей.

### atomic.Int32, atomic.Int64, atomic.Uint32, atomic.Uint64

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    var counter atomic.Int32

    // Не нужно передавать адрес, типы сами занимаются памятью
    counter.Store(10)
    counter.Add(5)

    fmt.Println(counter.Load()) // 15
}
```

Методы этих типов (без ...Int32):

- `Add(delta T) T` — добавить и вернуть новое значение
- `Load() T` — прочитать значение
- `Store(val T)` — записать значение
- `Swap(new T) T` — обменять и вернуть старое
- `CompareAndSwap(old, new T) bool` — CAS операция
- `And(mask T) T` — побитовое И
- `Or(mask T) T` — побитовое ИЛИ

Либо используем функции пакета atomic, либо методы переменной atomic.Int32

### atomic.Bool (Go 1.19+)

```go
package main

import (
    "fmt"
    "sync/atomic"
)

func main() {
    // Если не сломал голову - ты молодец
    // Если сломал - повтори CompareAndSwap
    var flag atomic.Bool

    flag.Store(true)
    fmt.Println(flag.Load()) // true

    swapped := flag.CompareAndSwap(true, false)
    fmt.Println(swapped) // true
    fmt.Println(flag.Load()) // false
}
```

### `atomic.Pointer[T]` (Go 1.19+, обобщенный тип)

Позволяет безопасно работать с указателями на любые типы без необходимости использовать `unsafe.Pointer`.

```go
package main

import (
    "fmt"
    "sync/atomic"
)

type Config struct {
    Host string
    Port int
}

func main() {
    var config atomic.Pointer[Config]

    cfg1 := &Config{"localhost", 8080}
    config.Store(cfg1)

    cfg2 := config.Load()
    fmt.Println(cfg2.Host) // localhost

    // Можно использовать CAS для замены конфига
    cfg3 := &Config{"0.0.0.0", 8080}
    config.CompareAndSwap(cfg1, cfg3)
}
```

### atomic.Value (для любых типов)

Более общий тип, работает с интерфейсами. Осторожнее: все значения, сохраняемые в один `Value`, должны быть одного типа!

```go
package main

import (
    "fmt"
    "sync/atomic"
)

type Config struct {
    Data string
}

func main() {
    var config atomic.Value

    config.Store(&Config{"initial"})

    // Чтение требует type assertion
    v := config.Load().(*Config)
    fmt.Println(v.Data) // initial

    // Ошибка: нельзя сохранять разные типы!
    // config.Store("string") — паника!
}
```

## Практические сценарии использования

### Сценарий 1: Конкурентный счетчик

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

type RequestCounter struct {
    count atomic.Int64
}

func (rc *RequestCounter) Increment() {
    rc.count.Add(1)
}

func (rc *RequestCounter) Get() int64 {
    return rc.count.Load()
}

func main() {
    counter := &RequestCounter{}
    var wg sync.WaitGroup

    // 100 горутин инкрементируют счетчик
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            for j := 0; j < 1000; j++ {
                counter.Increment()
            }
            wg.Done()
        }()
    }

    wg.Wait()
    fmt.Printf("Total requests: %d\n", counter.Get()) // 100000
}
```

### Сценарий 2: Флаг выключения (shutdown flag)

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

type Worker struct {
    shutdown atomic.Bool
}

func (w *Worker) Stop() {
    w.shutdown.Store(true)
}

func (w *Worker) IsRunning() bool {
    return !w.shutdown.Load()
}

func (w *Worker) Work() {
    for w.IsRunning() {
        fmt.Println("Working...")
        time.Sleep(100 * time.Millisecond)
    }
    fmt.Println("Stopped")
}

func main() {
    worker := &Worker{}

    go worker.Work()

    time.Sleep(500 * time.Millisecond)
    worker.Stop()

    time.Sleep(200 * time.Millisecond)
}
```

### Сценарий 3: Безопасное обновление конфига (copy-on-write)

```go
package main

import (
    "sync"
    "sync/atomic"
)

type Config map[string]string

type ConfigManager struct {
    config atomic.Pointer[Config]
    mu     sync.Mutex
}

func (cm *ConfigManager) Get(key string) string {
    cfg := cm.config.Load()
    if cfg != nil {
        return (*cfg)[key]
    }
    return ""
}

func (cm *ConfigManager) Update(key, value string) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    // Копируем старый конфиг
    oldCfg := cm.config.Load()
    newCfg := make(Config)

    if oldCfg != nil {
        for k, v := range *oldCfg {
            newCfg[k] = v
        }
    }

    // Обновляем
    newCfg[key] = value

    // Атомически заменяем весь конфиг
    cm.config.Store(&newCfg)
}

func main() {
    cm := &ConfigManager{}

    // Много читателей без блокировок
    for i := 0; i < 10; i++ {
        go func() {
            for {
                _ = cm.Get("host")
            }
        }()
    }

    // Редкое обновление с минимальной блокировкой
    cm.Update("host", "localhost")
}
```

### Сценарий 4: Retry логика с CAS

```go
package main

import (
    "fmt"
    "sync/atomic"
    "sync"
)

func incrementWithRetry(value *int32, maxRetries int) bool {
    for i := 0; i < maxRetries; i++ {
        current := atomic.LoadInt32(value)
        if atomic.CompareAndSwapInt32(value, current, current+1) {
            return true
        }
        // Если не получилось, пробуем снова
    }
    return false
}

func main() {
    var value int32 = 100
    var wg sync.WaitGroup

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            incrementWithRetry(&value, 5)
            wg.Done()
        }()
    }

    wg.Wait()
    fmt.Println("Final value:", value)
}
```

## Атомики vs Мьютексы: когда что использовать

| Аспект | Атомики | Мьютексы |
|--------|---------|---------|
| **Производительность** | Выше при низкой конкуренции | Лучше при высокой конкуренции |
| **Сложность кода** | Проще для примитивов | Нужна осторожность с блокировками |
| **Типы данных** | Только примитивы и указатели | Любые типы |
| **Масштабируемость** | Отлично масштабируется | Может быть узким местом |
| **Гарантии упорядочения** | Sequentially consistent | Варьируется в зависимости от реализации |

**Используйте атомики когда:**

- Работаете с простыми числовыми счетчиками
- Нужны флаги (true/false состояния)
- Нужна очень высокая производительность при частых чтениях
- Реализуете lock-free алгоритмы

**Используйте мьютексы когда:**

- Нужно защитить сложные структуры данных
- Логика требует сохранения инвариантов для нескольких полей
- Конкуренция за ресурс высокая
- Нужны более интуитивные гарантии синхронизации

## Важные детали реализации

### 64-битные операции требуют выравнивания

На 32-битных архитектурах 64-битные атомические операции требуют 8-байтового выравнивания. Типы `atomic.Int64` и `atomic.Uint64` автоматически выравниваются, но если вы используете функции вроде `atomic.AddInt64`, убедитесь в правильном выравнивании.

### Нельзя копировать значения после первого использования

Все типы `atomic.*` содержат скрытые поля и **не должны копироваться** после первого использования. Передавайте их только через указатели:

```go
// Правильно
var counter atomic.Int32
counter.Store(10)

func process(c *atomic.Int32) {
    c.Add(1)
}

process(&counter) // Передаем указатель

// Неправильно
process(counter) // Копирует значение — ошибка!
```

### atomic.Value требует консистентности типов

Все операции с одним `Value` должны использовать один и тот же конкретный тип:

```go
var v atomic.Value
v.Store("string")
v.Store(123) // Паника!

v.Load().(string) // OK
v.Load().(int)    // Паника!
```

## Частые вопросы на собеседованиях

### В чем разница между atomic.Load и просто прямым чтением переменной?

**Ответ:** Просто прочитанная переменная может привести к состоянию гонки, если она пишется из другой горутины. ЦПУ и компилятор могут переупорядочить операции, и вы можете получить частично обновленное значение. `atomic.Load` гарантирует:

- Чтение происходит за один индивидуальный шаг
- Устанавливается acquire semantics (барьер памяти)
- Вы никогда не получите "полуписанное" значение

```go
// Небезопасно
var x int32
go func() { x = 100 }()
// Может быть гонка

// Безопасно
var x atomic.Int32
go func() { x.Store(100) }()
v := x.Load() // Гарантированно корректное значение
```

### Что такое Compare-And-Swap и почему оно важно?

**Ответ:** CAS — это операция "если значение еще равно ожидаемому, то замени его". Это фундамент lock-free алгоритмов, потому что позволяет сделать условную замену атомически, без мьютекса:

```go
// В цикле пытаемся обновить значение
for {
    current := atomic.LoadInt32(&value)
    expected := current + 1
    if atomic.CompareAndSwapInt32(&value, current, expected) {
        break // Успешно обновили!
    }
    // Иначе повторяем (другая горутина изменила значение)
}
```

### В каких случаях атомики быстрее мьютексов?

**Ответ:** Атомики быстрее при:

- Низкой конкуренции (мало горутин одновременно обращаются)
- Простых операциях (просто инкремент, а не сложная логика)
- Очень частых операциях (микросекундный уровень)

Мьютексы становятся лучше при высокой конкуренции, потому что переключаются из режима со спин-локом в режим справедливой очереди без активного ожидания. Спин-лок используется только при низкой конкуренции как оптимизация.

### Что означает Sequential Consistency?

**Ответ:** Sequential Consistency гарантирует, что все атомические операции в программе ведут себя так, как если бы они выполнялись в одном глобальном порядке, видимом всем горутинам. Это самый строгий уровень гарантий памяти. Это означает:

- Не бывает "неожиданных" переупорядочиваний операций
- Полные барьеры памяти перед и после каждой операции
- Безопасно использовать для синхронизации

### Почему нельзя использовать атомики для сложных структур?

**Ответ:** Атомики работают только с примитивными типами (int32, int64, uint*, bool) и указателями. Для сложных структур:

- Невозможно атомически обновить несколько полей одновременно
- Нарушаются инварианты структуры
- Правильное решение — либо использовать мьютекс для защиты структуры, либо использовать copy-on-write с `atomic.Pointer`

### Какова разница между Load+Store и Swap?

**Ответ:**

- `Load` + `Store` — две отдельные операции, между ними может вмешаться другая горутина
- `Swap` — одна атомическая операция, гарантированно читает старое значение И пишет новое за один шаг

```go
// Неправильно (гонка между операциями)
old := atomic.LoadInt32(&x)
atomic.StoreInt32(&x, new)

// Правильно
old := atomic.SwapInt32(&x, new)
```

### Как правильно использовать atomic.Value?

**Ответ:**

- Сначала сохраните значение типа X
- Потом только с этим типом (тип должен быть одинаковым для всех операций)
- При чтении используйте type assertion
- Никогда не сохраняйте nil

```go
var config atomic.Value
config.Store(&Config{})  // Установили тип

c := config.Load().(*Config)  // Type assertion обязателен
```

### Безопасно ли передавать atomic через функции?

**Ответ:** Нельзя передавать копией! Только через указатель:

```go
// Неправильно
func Update(v atomic.Int32) {
    v.Add(1) // Обновляет копию, не оригинал!
}

// Правильно
func Update(v *atomic.Int32) {
    v.Add(1) // Обновляет оригинал
}
```

### Когда использовать And/Or операции?

**Ответ:** Используйте для управления отдельными битами (флаги, маски):

```go
const (
    FlagRunning  = 1 << 0 // бит 0
    FlagPaused   = 1 << 1 // бит 1
    FlagShutdown = 1 << 2 // бит 2
)

var flags atomic.Int32

// Установить флаг (OR с маской)
flags.Or(FlagRunning)

// Очистить флаг (AND с инвертированной маской)
flags.And(^FlagRunning)
```

### Могу ли я использовать атомики вместо каналов?

**Ответ:** Для синхронизации координации между горутинами каналы предпочтительнее. Атомики хороши для простого общего состояния. Каналы дают:

- Более понятный код
- Встроенную синхронизацию
- Возможность передачи значений между горутинами

Используйте атомики когда каналы будут избыточны (просто флаг или счетчик).

## Практика

### Задание 1: Счетчик посещений

Напишите многопоточный счетчик посещений веб-сервера. Несколько горутин будут увеличивать счетчик, и одна горутина будет периодически выводить текущее значение без блокировок.

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

// ЗАПОЛНИТЕ РЕАЛИЗАЦИЮ
type VisitorCounter struct {
    // ???
}

func (vc *VisitorCounter) Increment() {
    // ???
}

func (vc *VisitorCounter) GetCount() int64 {
    // ???
}

func main() {
    counter := &VisitorCounter{}
    var wg sync.WaitGroup

    // Симуляция посещений
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            for j := 0; j < 1000; j++ {
                counter.Increment()
                time.Sleep(1 * time.Millisecond)
            }
            wg.Done()
        }()
    }

    // Периодический вывод
    for i := 0; i < 10; i++ {
        fmt.Printf("Visitors: %d\n", counter.GetCount())
        time.Sleep(100 * time.Millisecond)
    }

    wg.Wait()
    fmt.Printf("Final: %d\n", counter.GetCount())
}
```

### Задание 2: Система включения/выключения

Реализуйте систему, которая может быть включена или выключена. Несколько горутин должны проверять, включена ли система, и множество горутин может переключать состояние без состояния гонки.

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

// ЗАПОЛНИТЕ РЕАЛИЗАЦИЮ
type System struct {
    // ???
}

func (s *System) Enable() {
    // ???
}

func (s *System) Disable() {
    // ???
}

func (s *System) IsEnabled() bool {
    // ???
}

func main() {
    sys := &System{}
    var wg sync.WaitGroup

    // Рабочие горутины
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(id int) {
            for j := 0; j < 20; j++ {
                if sys.IsEnabled() {
                    fmt.Printf("Worker %d: System is ON\n", id)
                } else {
                    fmt.Printf("Worker %d: System is OFF\n", id)
                }
                time.Sleep(50 * time.Millisecond)
            }
            wg.Done()
        }(i)
    }

    // Переключатель
    sys.Enable()
    time.Sleep(200 * time.Millisecond)
    sys.Disable()
    time.Sleep(200 * time.Millisecond)
    sys.Enable()

    wg.Wait()
}
```

### Задание 3: Конфигурационный менеджер с copy-on-write

Реализуйте менеджер конфигурации, который позволяет многим горутинам читать конфиг без блокировок и редко его обновлять.

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

type Config struct {
    Host string
    Port int
}

// ЗАПОЛНИТЕ РЕАЛИЗАЦИЮ
type ConfigManager struct {
    // ???
}

func (cm *ConfigManager) GetConfig() *Config {
    // ???
}

func (cm *ConfigManager) UpdateConfig(host string, port int) {
    // ???
}

func main() {
    cm := &ConfigManager{}
    cm.UpdateConfig("localhost", 8080)

    var wg sync.WaitGroup

    // Много читателей
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            for j := 0; j < 100; j++ {
                cfg := cm.GetConfig()
                if cfg != nil {
                    fmt.Printf("Reader %d: %s:%d\n", id, cfg.Host, cfg.Port)
                }
            }
            wg.Done()
        }(i)
    }

    // Редкие обновления
    cm.UpdateConfig("0.0.0.0", 9000)
    cm.UpdateConfig("example.com", 443)

    wg.Wait()
}
```

### Задание 4: Lock-free очередь попыток (retry queue)

Реализуйте простой lock-free счетчик попыток повтора с использованием CAS:

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

// ЗАПОЛНИТЕ РЕАЛИЗАЦИЮ
type RetryCounter struct {
    // ???
}

func (rc *RetryCounter) IncrementWithRetry(maxRetries int) bool {
    // ???
}

func (rc *RetryCounter) GetCount() int32 {
    // ???
}

func main() {
    counter := &RetryCounter{}
    var wg sync.WaitGroup
    successCount := 0

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            if counter.IncrementWithRetry(10) {
                successCount++
            }
            wg.Done()
        }()
    }

    wg.Wait()

    fmt.Printf("Final count: %d\n", counter.GetCount())
    fmt.Printf("Successful updates: %d\n", successCount)
}
```

## Решения практических заданий

### Решение 1: Счетчик посещений

```go
type VisitorCounter struct {
    count atomic.Int64
}

func (vc *VisitorCounter) Increment() {
    vc.count.Add(1)
}

func (vc *VisitorCounter) GetCount() int64 {
    return vc.count.Load()
}
```

### Решение 2: Система включения/выключения

```go
type System struct {
    enabled atomic.Bool
}

func (s *System) Enable() {
    s.enabled.Store(true)
}

func (s *System) Disable() {
    s.enabled.Store(false)
}

func (s *System) IsEnabled() bool {
    return s.enabled.Load()
}
```

### Решение 3: Менеджер конфигурации

```go
type ConfigManager struct {
    config atomic.Pointer[Config]
    mu     sync.Mutex
}

func (cm *ConfigManager) GetConfig() *Config {
    return cm.config.Load()
}

func (cm *ConfigManager) UpdateConfig(host string, port int) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    oldCfg := cm.config.Load()
    newCfg := &Config{Host: host, Port: port}
    cm.config.Store(newCfg)
}
```

### Решение 4: Lock-free retry counter

```go
type RetryCounter struct {
    count atomic.Int32
}

func (rc *RetryCounter) IncrementWithRetry(maxRetries int) bool {
    for i := 0; i < maxRetries; i++ {
        current := rc.count.Load()
        if rc.count.CompareAndSwap(current, current+1) {
            return true
        }
    }
    return false
}

func (rc *RetryCounter) GetCount() int32 {
    return rc.count.Load()
}
```

## Резюме ключевых моментов

1. **Индивидуальность:** Каждая атомическая операция выполняется за один неделимый шаг на уровне ЦПУ

2. **Типы:** Go предоставляет `Int32`, `Int64`, `Uint32`, `Uint64`, `Bool`, `Pointer[T]` и `Value`

3. **Основные операции:** Load, Store, Add, Swap, CompareAndSwap, And, Or

4. **Sequential Consistency:** Go гарантирует самый строгий уровень упорядочения памяти

5. **Когда использовать:** Просты числовые счетчики, флаги, или performance-critical простые операции

6. **Когда не использовать:** Сложные структуры, множество связанных полей, частые обновления большого объема данных

7. **Не копируйте:** После первого использования не передавайте `atomic.*` по значению, только по указателю

8. **Современные типы:** Предпочитайте `atomic.Int32` и друзей функциям вроде `atomic.AddInt32` — удобнее и безопаснее
