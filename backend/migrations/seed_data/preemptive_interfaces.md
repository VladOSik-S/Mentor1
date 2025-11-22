---
title: "Preemptive Interfaces"
tags:
  - интерфейс
is_public: true 
# preview: "..."
---

**Preemptive interfaces** — это антипаттерн в Go, когда интерфейс пишется до того как становится понятно, что он действительно нужен.

## Что такое preemptive interfaces

Preemptive interfaces возникают, когда разработчик создает интерфейс слишком рано в процессе разработки, еще до того как стал понятен реальный способ использования. Это особенно характерно для разработчиков, приходящих из Java или C\#, где интерфейсы являются обязательной частью архитектуры.

### Пример антипаттерна

```go
// Плохой пример - preemptive interface
type FooInterface interface {
    DoFoo()
    DoBar()
}

type fooImpl struct{
    X int
}

func New(x int) FooInterface {
    return &fooImpl{X:x}
}

func (f *fooImpl) DoFoo() {}
func (f *fooImpl) DoBar() {}
```

В этом примере интерфейс определен производителем API и возвращается из конструктора, что **создает преждевременную абстракцию**.

## Принцип "Accept interfaces, return structs"

Вместо preemptive interfaces в Go рекомендуется следовать принципу **"Accept interfaces, return structs"**.

### Правильный подход

```go
// Хорошая практика
type fooImpl struct{
    X int
}

func New(x int) *fooImpl {
    return &fooImpl{X:x}
}

func (f *fooImpl) DoFoo() {}
func (f *fooImpl) DoBar() {}

// Интерфейс определяется потребителем
type FooDoer interface {
    DoFoo()
}

func ProcessFoo(f FooDoer) {
    f.DoFoo()
}
```

### Преимущества такого подхода

1. **Гибкость**: Функция принимает любой тип, реализующий нужные методы
2. **Тестируемость**: Легко создавать моки и заглушки
3. **Расширяемость**: Структуры можно переиспользовать с разными интерфейсами
4. **Минимальные интерфейсы**: Интерфейсы содержат только необходимые методы

## Неявная реализация интерфейсов (Implicit interfaces)

Go использует **structural typing** с неявной реализацией интерфейсов. Это означает, что если тип имеет все методы интерфейса, он автоматически его реализует.

```go
type Duck interface {
    Quack()
    Swim()
}

type Mallard struct{}
func (m Mallard) Quack() { println("quack") }
func (m Mallard) Swim()  { println("swimming") }

// Mallard автоматически реализует Duck
var duck Duck = Mallard{}
```

### Duck typing vs Structural typing

Хотя Go часто называют языком с duck typing, технически это **structural typing**, поскольку проверка типов происходит во время компиляции, а не выполнения. Ну это так, мем, будет скучно на собесе загасишь интервьюера. Про утиную типизацию говорить все равно надо.

## Преимущества маленьких интерфейсов

В Go действует принцип: чем меньше интерфейс, тем он полезнее. Самый мощный интерфейс в стандартной библиотеке — это `io.Reader` с одним методом:

```go
type Reader interface {
    Read([]byte) (n int, err error)
}
```

Статистика стандартной библиотеки показывает, что интерфейсы с 1-2 методами встречаются в три раза чаще всех остальных.

## Когда preemptive interfaces допустимы

Есть случаи, когда preemptive interfaces могут быть оправданы:

1. Когда действительно существует несколько реализаций (например, пакет `hash` в стандартной библиотеке)
2. Для сокрытия деталей реализации в больших системах
3. При разработке библиотек с четко определенными контрактами

## Частые ошибки при работе с интерфейсами

### Слишком большие интерфейсы

Создание интерфейсов с множеством методов нарушает принцип единственной ответственности/

### Возврат интерфейсов вместо структур

Это ограничивает возможности вызывающего кода и создает ненужную абстракцию.

### Неправильное понимание nil интерфейсов

```go
var i interface{} = (*int)(nil)
fmt.Println(i == nil) // false! - частая ошибка на собеседованиях
```

Интерфейс содержит тип и значение. Даже если значение nil, тип не nil.

## Вопросы для собеседований

### Теоретические вопросы

1. **Что такое preemptive interfaces и почему это антипаттерн?**
    - Объяснить принцип "интерфейсы определяются потребителем"
    - Привести пример правильного и неправильного подхода
2. **В чем разница между duck typing и structural typing в Go?**
    - Structural typing проверяется на этапе компиляции
    - Duck typing — динамическая проверка во время выполнения
3. **Почему в Go рекомендуется "Accept interfaces, return structs"?**
    - Гибкость для вызывающего кода
    - Возможность тестирования
    - Избежание преждевременной абстракции
4. **Когда interface{} равен nil?**
    - Только когда и тип, и значение равны nil
    - Объяснить внутреннюю структуру интерфейса

### Практические задания

#### Задание 1: Рефакторинг preemptive interface

Дан код с preemptive interface:

```go
type Service interface {
    Process(data string) string
    Validate(data string) bool
    Save(data string) error
}

type serviceImpl struct{}

func NewService() Service {
    return &serviceImpl{}
}

func (s *serviceImpl) Process(data string) string { return "processed" }
func (s *serviceImpl) Validate(data string) bool { return true }
func (s *serviceImpl) Save(data string) error    { return nil }
```

**Задача**: Переписать согласно принципам Go.

**Решение**:

```go
type Service struct{}

func NewService() *Service {
    return &Service{}
}

func (s *Service) Process(data string) string { return "processed" }
func (s *Service) Validate(data string) bool { return true }
func (s *Service) Save(data string) error    { return nil }

// Интерфейсы определяются потребителями
type Processor interface {
    Process(string) string
}

type Validator interface {
    Validate(string) bool
}

func HandleData(p Processor, v Validator, data string) string {
    if v.Validate(data) {
        return p.Process(data)
    }
    return ""
}
```

#### Задание 2: Nil интерфейс

```go
func mystery() interface{} {
    var p *int
    return p
}

func main() {
    result := mystery()
    if result == nil {
        fmt.Println("result is nil")
    } else {
        fmt.Println("result is not nil")
    }
}
```

**Вопрос**: Что выведет программа и почему?

**Ответ**: "result is not nil", потому что интерфейс содержит тип `*int` и значение `nil`, но сам интерфейс не nil.

#### Задание 3: Создание минимального интерфейса

Дана структура:

```go
type User struct {
    ID   int
    Name string
    Email string
}

func (u User) GetID() int       { return u.ID }
func (u User) GetName() string  { return u.Name }
func (u User) GetEmail() string { return u.Email }
func (u User) IsValid() bool    { return u.Email != "" }
```

**Задача**: Создать функцию для отправки уведомлений, используя минимальный интерфейс.

**Решение**:

```go
type Notifiable interface {
    GetEmail() string
}

func SendNotification(n Notifiable, message string) error {
    // Отправка уведомления на n.GetEmail()
    return nil
}
```

## Заключение

Preemptive interfaces — это антипаттерн, который следует избегать в Go. Правильный подход заключается в том, чтобы интерфейсы определялись потребителями API, а не производителями. Это делает код более гибким, тестируемым и соответствующим философии языка Go, о чем ты и мечтаешь.
