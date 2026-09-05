/**
 * Словарь строк интерфейса — источник истины для всех надписей (frontend-plan §3).
 * Строки из брифа взяты дословно — их менять нельзя.
 * Каркасные строки FE-0 собраны в SCAFFOLD и удаляются по мере появления фич
 * (зафиксировано в .agent/ui-spec.md §6).
 */

export const JOB_LABEL = {
  queued: 'В очереди',
  running: 'Выполняется',
  completed: 'Завершён',
  failed: 'Ошибка',
  cancelled: 'Отменён',
} as const;

export const VERDICT_LABEL = {
  normal: 'Негативных отклонений не выявлено',
  candidate: 'Возможное изменение',
  confirmed: 'Подтверждённое изменение',
  insufficient_data: 'Недостаточно данных',
} as const;

export const PROVENANCE_LABEL = {
  observed: 'Наблюдение',
  imputed: 'Восстановлено',
  missing: 'Нет данных',
} as const;

// Ключи стадий согласовать с B4; неизвестная → 'Анализ выполняется' (corrections §1)
export const STAGE_LABEL = {
  collect_satellite: 'Получение спутниковых данных',
  collect_weather: 'Получение погоды',
  prepare_input: 'Подготовка данных',
  analyze: 'Анализ',
  save_result: 'Сохранение результата',
} as const;

export const EMPTY = {
  noAreas:
    'Участков пока нет. Найдите сельхозконтуры в видимой области карты или нарисуйте участок.',
  contoursNotFound: 'Контуры не найдены в этой области',
  contoursFailed: 'Не удалось получить контуры',
  noSatellite: 'Нет пригодных спутниковых данных за выбранный период',
  littleHistory: 'Недостаточно истории для сравнения с сезонным фоном',
  connectionLost: 'Нет связи, состояние задачи неизвестно',
  weatherPartial: 'Погода доступна не на весь период',
  causeUnknown: 'Причина по доступным данным не установлена',
  deleteActive: 'Участок будет удалён. Результат текущего анализа не будет сохранён.',
  demo: 'Демонстрационные данные',
} as const;

/**
 * Блоки карточки события (design-brief §4) — дословно из брифа.
 * На FE-1 используются на dev-странице состояний, на FE-5 — в EventCard.
 */
export const EVENT_BLOCK_LABEL = {
  detected: 'Что обнаружено',
  basis: 'На чём основано',
  weather: 'Погодный контекст и гипотеза',
  limitations: 'Ограничения',
} as const;

/** Стадия задачи, которую backend не передал или передал неизвестную (frontend-plan §3). */
export const JOB_STAGE_FALLBACK = 'Анализ выполняется';

/**
 * Строки dev-страницы состояний (?dev=states, только import.meta.env.DEV).
 * Технические подписи таблиц; в пользовательский интерфейс не попадают.
 */
export const DEV_LABELS = {
  title: 'Состояния и фикстуры (dev)',
  areasTable: 'Участки',
  seriesTable: 'Точки ряда',
  eventsList: 'События',
  jobBlock: 'Задача (живой сценарий)',
  selectArea: 'Участок',
  runAnalysis: 'Запустить анализ',
  runQueueFull: 'Проверить 429',
  columnSource: 'Источник',
  columnVerdict: 'Вывод',
  columnSeverity: 'Тяжесть',
  columnPeriod: 'Период результата',
  columnJob: 'Задача',
  columnDate: 'Дата',
  columnNdvi: 'NDVI',
  columnProvenance: 'Происхождение',
  columnBackground: 'Фон (среднее)',
  columnDeviation: 'Отклонение',
  dash: '—',
  error: 'Ошибка',
} as const;

/**
 * Строки карты и рисования (FE-3). Тексты состояний ContoursButton — из плана §6.4,
 * сообщения валидации геометрии — из corrections §1 (самопересечение/лимиты).
 */
export const MAP_LABELS = {
  searching: 'Ищем контуры…',
  stale: 'Область изменилась — искать снова',
  contourSource: 'Контур OpenStreetMap',
  drawHint: 'Обведите нужную область одной непрерывной линией',
  undoVertex: 'Отменить точку',
  finishDraw: 'Завершить',
  cancelDraw: 'Отмена',
  vertexCount: (n: number) => `Вершин: ${n}`,
  basemapToMap: 'Карта',
  basemapToSatellite: 'Спутник',
  legend: 'Легенда',
  legendNone: 'Не анализировался',
  legendContour: 'Найденный контур',
  legendSelected: 'Выбранный участок',
  areaHa: (v: string) => `${v} га`,
  selfIntersection: 'Полигон самопересекается',
  tooFewVertices: 'Нужно минимум 3 вершины',
  areaLimit: (max: number) => `Площадь больше ${max} га`,
  verticesLimit: (max: number) => `Слишком много вершин: максимум ${max}`,
  periodLimit: (max: number) => `Период длиннее ${max} дней`,
  badPeriod: 'Дата «с» позже даты «по»',
  contoursCount: (n: number) => {
    const mod10 = n % 10;
    const mod100 = n % 100;
    const word =
      mod10 === 1 && mod100 !== 11
        ? 'контур'
        : mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)
          ? 'контура'
          : 'контуров';
    return `${n} ${word}`;
  },
} as const;

/** Строки диалога добавления участка (FE-3, бриф §3A). */
export const DIALOG_LABELS = {
  name: 'Название',
  nameRequired: 'Укажите название участка',
  period: 'Период',
  from: 'с',
  to: 'по',
  source: 'Источник',
  area: 'Площадь',
  addAndAnalyze: 'Добавить и проанализировать',
  saveOnly: 'Только сохранить',
} as const;

/** Строки минимального списка участков (FE-3; полноценный список — FE-2). */
export const AREA_LIST_LABELS = {
  notAnalyzed: 'Не анализировался',
  runAnalysis: 'Запустить анализ',
} as const;

/**
 * SCAFFOLD (FE-0): строки каркаса — заголовки панелей, вкладки, тексты зон-заглушек
 * и aria-подписи. Не являются утверждённым словарём брифа; заменяются на этапах FE-1–FE-5.
 */
export const SCAFFOLD = {
  appTitle: 'TerraLens',
  appSubtitle: 'Аналитика полей',
  areasPanel: 'Участки',
  mapPanel: 'Карта',
  analysisPanel: 'Анализ',
  addArea: 'Добавить',
  addAreaLong: 'Добавить участок',
  noAreaSelected: 'Участок не выбран',
  periodNotSelected: 'Период не выбран',
  runAnalysis: 'Запустить анализ',
  findContours: 'Найти контуры в этой области',
  drawArea: 'Нарисовать участок',
  openAreasList: 'Открыть список участков',
  openAreaCard: 'Открыть карточку участка',
  mainNavigation: 'Основная навигация',
  mobileTabAreas: 'Участки',
  mobileTabMap: 'Карта',
  mobileTabAnalysis: 'Анализ',
  // Тексты зон-заглушек; исчезают на этапах FE-2–FE-4
  placeholderMap: 'Здесь будет карта',
  placeholderChart: 'Здесь будет график NDVI',
  placeholderWeather: 'Здесь будут погода и объяснение',
  placeholderCard: 'Здесь будет карточка участка',
  placeholderList: 'Список участков',
} as const;
