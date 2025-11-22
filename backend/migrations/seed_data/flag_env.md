---
title: "Флаги и переменные окружения"
tags: []
is_public: true 
# preview: "..."
---

## Что такое флаги командной строки

Флаги — это параметры, которые передаются программе при её запуске через командную строку. Они позволяют изменять поведение программы без изменения исходного кода. В Go для работы с флагами используется стандартный пакет `flag`.

### Основные концепции работы с флагами

Пакет `flag` предоставляет функции для определения и парсинга флагов различных типов: строки, целые числа, булевы значения, длительности времени и другие. Каждая функция возвращает указатель на значение флага.

### Базовый пример использования

```
package main

import (
    "flag"
    "fmt"
)

func main() {
    name := flag.String("name", "World", "имя для приветствия")
    age := flag.Int("age", 0, "возраст пользователя")
    verbose := flag.Bool("verbose", false, "включить подробный вывод")

    flag.Parse()
    
    fmt.Printf("Привет, %s!\n", *name)
    fmt.Printf("Возраст: %d\n", *age)
    fmt.Printf("Режим подробного вывода: %v\n", *verbose)
}
```

Программа запускается так: `go run main.go -name=Иван -age=25 -verbose`.

### Функция flag.Parse()

Функция `flag.Parse()` выполняет парсинг аргументов командной строки и присваивает значения флагам. Критически важно вызывать её после определения всех флагов, но до их использования.

### Два варианта определения флагов

Существует два подхода к определению флагов:

Первый вариант возвращает указатель на переменную:

```

port := flag.Int("port", 8080, "номер порта")

```

Второй вариант принимает указатель на существующую переменную:

```

var port int
flag.IntVar(&port, "port", 8080, "номер порта")

```

Разница в том, что в первом случае пакет сам создаёт переменную, а во втором мы передаём уже существующую.

### Синтаксис флагов

Go поддерживает несколько вариантов записи флагов:

Для обычных типов: `-port=8080`, `-port 8080`, `--port=8080`, `--port 8080`.

Для булевых флагов: `-verbose`, `--verbose`, `-verbose=true`, `--verbose=false`.

### Работа с позиционными аргументами

После вызова `flag.Parse()` можно получить доступ к позиционным аргументам (не-флагам) через функции `flag.Args()` и `flag.Arg(i)`. Функция `flag.Args()` возвращает срез всех позиционных аргументов, а `flag.Arg(i)` возвращает i-й аргумент.

```

flag.Parse()
positionalArgs := flag.Args()
fmt.Println("Позиционные аргументы:", positionalArgs)

```

### Создание пользовательских типов флагов

Для создания флагов с пользовательской логикой необходимо реализовать интерфейс `flag.Value`. Этот интерфейс требует двух методов: `String() string` и `Set(string) error`.

```

type arrayFlags []string

func (i *arrayFlags) String() string {
    return fmt.Sprintf("%v", *i)
}

func (i *arrayFlags) Set(value string) error {
    *i = append(*i, value)
    return nil
}

var languages arrayFlags
flag.Var(\&languages, "lang", "языки программирования")

```

Этот подход позволяет создавать флаги, принимающие множественные значения или требующие валидации.

### FlagSet и подкоманды

Тип `FlagSet` позволяет создавать независимые наборы флагов, что полезно для реализации подкоманд. Каждый `FlagSet` имеет собственное пространство флагов и может парситься отдельно.

```

createCmd := flag.NewFlagSet("create", flag.ExitOnError)
name := createCmd.String("name", "", "имя объекта")

deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
force := deleteCmd.Bool("force", false, "принудительное удаление")

switch os.Args {
    case "create":
    createCmd.Parse(os.Args[2:])
    case "delete":
    deleteCmd.Parse(os.Args[2:])
}

```

### Стратегии обработки ошибок

При создании `FlagSet` можно указать одну из трёх стратегий обработки ошибок:

`flag.ContinueOnError` — продолжает парсинг даже при ошибке.

`flag.ExitOnError` — завершает программу при ошибке.

`flag.PanicOnError` — вызывает панику при ошибке.

## Переменные окружения

Переменные окружения — это пары ключ-значение, которые хранятся вне кода приложения на уровне операционной системы. Они используются для конфигурирования приложений без изменения исходного кода.

### Чтение переменных окружения

Для работы с переменными окружения используется пакет `os`. Основная функция — `os.Getenv(key string)`, которая возвращает значение переменной окружения или пустую строку, если переменная не установлена.

```

package main

import (
    "fmt"
    "os"
)

func main() {
    dbHost := os.Getenv("DB_HOST")
    if dbHost == "" {
        dbHost = "localhost" // значение по умолчанию
    }
    fmt.Println("Database host:", dbHost)
}

```

### Функция os.LookupEnv

Функция `os.LookupEnv(key string)` возвращает два значения: само значение переменной и булево значение, указывающее, была ли переменная установлена. Это позволяет различать пустое значение и отсутствие переменной.

```

val, ok := os.LookupEnv("API_KEY")
if !ok {
    fmt.Println("API_KEY не установлен")
} else {
    fmt.Printf("API_KEY=%s\n", val)
}

```

### Установка переменных окружения

Функция `os.Setenv(key, value string)` устанавливает переменную окружения в рамках текущего процесса. Изменения видны только в текущей программе и дочерних процессах.

```

os.Setenv("APP_ENV", "production")

```

### Получение всех переменных окружения

Функция `os.Environ()` возвращает срез всех переменных окружения в формате "KEY=value".

```

for _, env := range os.Environ() {
fmt.Println(env)
}

```

### Работа с .env файлами

В разработке часто используются `.env` файлы для хранения переменных окружения. Этот файлик добавляют в гитигнор, а в нем прописывают переменные, которые нельзя хранить в коде, например пароли. Библиотека `github.com/joho/godotenv` загружает переменные из файла в окружение.

```

package main

import (
    "log"
    "os"
    "github.com/joho/godotenv"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Ошибка загрузки .env файла")
    }

    dbHost := os.Getenv("DB_HOST")
    fmt.Println("DB Host:", dbHost)
}

```

Файл `.env` содержит переменные в формате:

```

DB_HOST=localhost
DB_PORT=5432
API_KEY=secret_key_123

```

### Библиотека Viper для конфигурации

Viper — это мощная библиотека для управления конфигурацией, поддерживающая множество источников: файлы конфигурации, переменные окружения, флаги командной строки. **Вообще она тяжелая и неоднозначный взгляд у сообщества на нее, но знать что такая есть надо**. Она автоматически читает конфигурацию из разных источников с приоритетами.

```

package main

import (
    "fmt"
    "github.com/spf13/viper"
)

func main() {
    viper.SetConfigFile(".env")
    viper.ReadInConfig()

    dbHost := viper.GetString("DB_HOST")
    dbPort := viper.GetInt("DB_PORT")
    
    fmt.Printf("DB: %s:%d\n", dbHost, dbPort)
}

```

## Флаги vs Переменные окружения

### Когда использовать флаги

Флаги удобны для параметров, которые часто меняются при каждом запуске программы. Они обеспечивают явную и понятную конфигурацию прямо в командной строке.

> Я флагов в коммерческом коде не видел.

### Когда использовать переменные окружения

Переменные окружения подходят для конфигурации, зависящей от среды выполнения: development, staging, production. Они удобны для хранения чувствительных данных, таких как пароли и API-ключи.

### Комбинированный подход

Распространённая практика — использовать переменные окружения как значения по умолчанию для флагов. Флаги имеют приоритет над переменными окружения, что позволяет переопределять конфигурацию при запуске.

```

package main

import (
    "flag"
    "fmt"
    "os"
)

func main() {
    defaultPort := os.Getenv("APP_PORT")
    if defaultPort == "" {
        defaultPort = "8080"
    }

    port := flag.String("port", defaultPort, "порт сервера")
    flag.Parse()
    
    fmt.Printf("Сервер запущен на порту: %s\n", *port)
}
```

### Best practices

Не хардкодить секреты в коде — всегда использовать переменные окружения или внешние хранилища.

Предоставлять значения по умолчанию для всех конфигурационных параметров.

Валидировать значения флагов и переменных окружения при старте приложения.

Использовать явные имена для флагов и переменных окружения.

Документировать все доступные флаги через параметр usage.

## Частые вопросы на собеседованиях

### Базовые вопросы

**Что такое пакет flag и для чего он используется?**
Пакет flag — стандартный пакет Go для парсинга аргументов командной строки. Он позволяет определять флаги разных типов, парсить их и получать значения для конфигурирования программы.

**В чём разница между flag.String и flag.StringVar?**
`flag.String()` создаёт и возвращает указатель на новую переменную, а `flag.StringVar()` принимает указатель на существующую переменную и записывает значение в неё.

**Зачем нужен вызов flag.Parse()?**
`flag.Parse()` выполняет парсинг аргументов командной строки и присваивает значения определённым флагам. Без этого вызова флаги будут иметь только значения по умолчанию.

**Что возвращает os.Getenv(), если переменная окружения не установлена?**
`os.Getenv()` возвращает пустую строку, если переменная не установлена. Для проверки существования переменной используется `os.LookupEnv()`.

### Продвинутые вопросы

**Как создать флаг с пользовательским типом?**
Необходимо реализовать интерфейс `flag.Value`, который требует методы `String() string` и `Set(string) error`. Затем использовать `flag.Var()` для регистрации флага.

**Что такое FlagSet и когда его использовать?**
`FlagSet` — это независимый набор флагов, используемый для создания подкоманд или парсинга флагов из разных источников[web:22][web:23]. Каждый `FlagSet` имеет собственное пространство имён флагов[web:23].

**Какие стратегии обработки ошибок существуют в пакете flag?**
Три стратегии: `ContinueOnError` (продолжить при ошибке), `ExitOnError` (завершить программу), `PanicOnError` (вызвать панику).

**Как получить позиционные аргументы после флагов?**
После вызова `flag.Parse()` использовать `flag.Args()` для получения среза всех позиционных аргументов или `flag.Arg(i)` для получения конкретного аргумента.

**В чём разница между os.Getenv и os.LookupEnv?**
`os.Getenv()` возвращает только значение, `os.LookupEnv()` возвращает значение и булев флаг существования переменной. Второй вариант позволяет различать пустое значение и отсутствие переменной.

**Как комбинировать флаги и переменные окружения?**
Использовать переменные окружения как значения по умолчанию при определении флагов. Флаги будут иметь приоритет и смогут переопределять значения из окружения.

**Что такое godotenv и зачем она нужна?**
`godotenv` — библиотека для загрузки переменных окружения из `.env` файлов. Она упрощает управление конфигурацией в разработке, избегая необходимости экспортировать переменные вручную.

**Когда следует использовать Viper вместо стандартных пакетов?**
Viper подходит для сложных сценариев конфигурации с множеством источников: файлы разных форматов, переменные окружения, флаги, удалённые системы конфигурации. Для простых случаев достаточно стандартных пакетов.

## Практические задания

### Задание 1: Базовый парсинг флагов

Создайте программу, которая принимает флаги `-host`, `-port` и `-verbose`. Программа должна выводить конфигурацию подключения и дополнительную информацию, если включён verbose режим.

```

package main

import (
    "flag"
    "fmt"
)

func main() {
    host := flag.String("host", "localhost", "хост для подключения")
    port := flag.Int("port", 8080, "порт для подключения")
    verbose := flag.Bool("verbose", false, "подробный вывод")

    flag.Parse()
    
    fmt.Printf("Подключение к %s:%d\n", *host, *port)
    
    if *verbose {
        fmt.Println("Режим подробного вывода включён")
        fmt.Println("Версия программы: 1.0.0")
    }
}
```

Запуск: `go run main.go -host=example.com -port=3000 -verbose`

### Задание 2: Работа с переменными окружения

Создайте программу, читающую конфигурацию базы данных из переменных окружения с fallback на значения по умолчанию.

```

package main

import (
    "fmt"
    "os"
    "strconv"
)

type DBConfig struct {
    Host     string
    Port     int
    Database string
    User     string
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}

func main() {
    config := DBConfig{
        Host:     getEnv("DB_HOST", "localhost"),
        Port:     getEnvInt("DB_PORT", 5432),
        Database: getEnv("DB_NAME", "myapp"),
        User:     getEnv("DB_USER", "postgres"),
    }

    fmt.Printf("Database Config:\n")
    fmt.Printf("Host: %s\n", config.Host)
    fmt.Printf("Port: %d\n", config.Port)
    fmt.Printf("Database: %s\n", config.Database)
    fmt.Printf("User: %s\n", config.User)
}
```

Запуск: `DB_HOST=production.db DB_PORT=5433 go run main.go`

### Задание 3: Комбинирование флагов и переменных окружения

Создайте приложение, которое использует переменные окружения как значения по умолчанию, но позволяет переопределить их флагами.

```

package main

import (
    "flag"
    "fmt"
    "os"
    "strconv"
)

func main() {
    defaultPort := "8080"
    if envPort := os.Getenv("APP_PORT"); envPort != "" {
        defaultPort = envPort
    }

    defaultHost := "localhost"
    if envHost := os.Getenv("APP_HOST"); envHost != "" {
        defaultHost = envHost
    }
    
    host := flag.String("host", defaultHost, "хост сервера")
    portStr := flag.String("port", defaultPort, "порт сервера")
    
    flag.Parse()
    
    port, err := strconv.Atoi(*portStr)
    if err != nil {
        fmt.Printf("Ошибка: некорректный порт %s\n", *portStr)
        os.Exit(1)
    }
    
    fmt.Printf("Сервер запущен на %s:%d\n", *host, port)
}
```

Запуск с переменными: `APP_HOST=0.0.0.0 APP_PORT=3000 go run main.go`
Запуск с флагами: `go run main.go -host=127.0.0.1 -port=9000`
Комбинация: `APP_PORT=3000 go run main.go -host=192.168.1.1`

### Задание 4: Создание пользовательского типа флага

Реализуйте флаг, который принимает список значений через запятую и сохраняет их в срез.

```

package main

import (
    "flag"
    "fmt"
    "strings"
)

type StringSlice []string

func (s *StringSlice) String() string {
    return strings.Join(*s, ",")
}

func (s *StringSlice) Set(value string) error {
    *s = strings.Split(value, ",")
return nil
}

func main() {
    var tags StringSlice
    flag.Var(\&tags, "tags", "список тегов через запятую")

    flag.Parse()
    
    fmt.Println("Теги:")
    for i, tag := range tags {
        fmt.Printf("%d: %s\n", i+1, tag)
    }
}
```

Запуск: `go run main.go -tags=golang,backend,api`

### Задание 5: Использование godotenv

Создайте приложение, загружающее конфигурацию из `.env` файла.

Сначала установите библиотеку: `go get github.com/joho/godotenv`

Создайте файл `.env`:

```
APP_NAME=MyApplication
APP_VERSION=1.0.0
DB_HOST=localhost
DB_PORT=5432
DEBUG=true
```

Код программы:

```
package main

import (
    "fmt"
    "log"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Ошибка загрузки .env файла")
    }

    appName := os.Getenv("APP_NAME")
    appVersion := os.Getenv("APP_VERSION")
    dbHost := os.Getenv("DB_HOST")
    dbPort := os.Getenv("DB_PORT")
    
    debug := false
    if debugStr := os.Getenv("DEBUG"); debugStr == "true" {
        debug = true
    }
    
    fmt.Printf("%s v%s\n", appName, appVersion)
    fmt.Printf("База данных: %s:%s\n", dbHost, dbPort)
    fmt.Printf("Режим отладки: %v\n", debug)
}
```

Запуск: `go run main.go`
