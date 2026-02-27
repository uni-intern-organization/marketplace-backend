# Импорт запросов в Insomnia

## Вариант 1: Импорт JSON-коллекции

1. Открой Insomnia.
2. Меню **Application** → **Preferences** → **Data** (или **Import/Export**).
3. Либо: в левой панели нажми **+** или **Create** → **Import** → **From File**.
4. Укажи файл:  
   `marketplace-backend/docs/insomnia_collection.json`
5. Если формат не подошёл (другая версия Insomnia), используй вариант 2.

После импорта:
- В окружении **Base** задай `base_url`: `http://localhost:8081`.
- После логина (POST login) скопируй `token` из ответа и вставь в переменную `token` в окружении — тогда все запросы из папки «Protected» будут отправляться с этим токеном (Bearer).
- Для **POST files/resume** и **POST files/logo**: открой запрос → Body → выбери **Multipart Form** → добавь поле **file** (или **resume**) / **logo** типа **File** и выбери файл.

---

## Вариант 2: Создать запросы вручную

Все URL, методы и примеры тел есть в:
- **docs/REQUEST_FORMATS.md** — формат тела (JSON / form-data).
- **docs/API_REQUESTS.md** — полное описание и примеры ответов.

Кратко:
- **Base URL:** `http://localhost:8081`
- Защищённые запросы: вкладка **Auth** → **Bearer Token** → вставь токен из POST login.
- JSON-запросы: Body → **JSON**, вставь пример из REQUEST_FORMATS.md.
- Загрузка резюме: Body → **Multipart**, ключ `file` или `resume`, тип **File**, выбери PDF.
- Загрузка логотипа: Body → **Multipart**, ключ `logo`, тип **File**, выбери картинку.

---

## Порядок проверки

1. **GET health** — убедиться, что API доступен.
2. **POST register** — зарегистрировать студента/рекрутера.
3. **POST login** — взять `token`, подставить в окружение или в Auth.
4. **GET me** — проверить профиль.
5. **PUT me/profile** или **PUT me/recruiter** — заполнить профиль.
6. **POST files/resume** (Multipart, поле `file`) — загрузить резюме.
7. Остальные запросы — по необходимости.
