![Архитекутра](https://github.com/dimuls/camtester/blob/master/architecture.png)

# camtester

Распределённый масштабируемый сервис детекций отклонений в видеоданных.

Данный репозиторий включает все компоненты системы кроме `restreamer`, который
находится в [отдельном репозитории](https://github.com/dimuls/rtsp-simple-proxy).

Краткое описание Go-пакетов и папок этого репозитория приведены ниже.

## [checker](https://github.com/dimuls/camtester/tree/master/checker)
Пакет ядра модуля тестирования видеопотока путём парсинга вывода `ffmpeg`.

## [cmd](https://github.com/dimuls/camtester/tree/master/cmd)
Содержит исходные коды программ, которые непосредственно компилируются в
исполняемые файлы.

## [core](https://github.com/dimuls/camtester/tree/master/core)
Пакет ядра `core` - основного компонента системы, который принимает задачи
по REST API и, обрабатывает результаты задач.

## [deployments](https://github.com/dimuls/camtester/tree/master/deployments)
Содержит Dockerfile компонентов системы и `docker-compose.yml` для запуска
тестовой сборки системы. Для сборки и запуска требуется что бы в соответствующих
подкаталогах были скомпилированные версии модулей.

## [entity](https://github.com/dimuls/camtester/tree/master/entity)
Пакет с основными сущностями системы.

## [ffmpeg](https://github.com/dimuls/camtester/tree/master/ffmpeg)
Пакет для работы с программами `ffmpeg` и `ffprobe`

## [http](https://github.com/dimuls/camtester/tree/master/http)
Пакет для работы с HTTP. Cодержит клиент для `restreamer-provider`.

## [nats](https://github.com/dimuls/camtester/tree/master/nats)
Пакет для работы с nats. Содержит клиенты для получения тасков и результатов
тасков, а так-же клиенты для отправки тасков и результатов тасков.

## [pinger](https://github.com/dimuls/camtester/tree/master/pinger)
Пакет ядра модуля пинга видеокамер.

## [prober](https://github.com/dimuls/camtester/tree/master/prober)
Пакет ядра модуля пробинга видеопотока путём парсинга вывода `ffprobe`.

## [redis](https://github.com/dimuls/camtester/tree/master/redis)
Пакет для работы с redis. Содержит реализацию интерфейса `core.DBStorage`.
