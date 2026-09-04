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
  satellite: 'Получение спутниковых данных',
  weather: 'Получение погоды',
  prepare: 'Подготовка данных',
  analysis: 'Анализ',
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
 * SCAFFOLD (FE-0): строки каркаса — заголовки панелей, вкладки, тексты зон-заглушек
 * и aria-подписи. Не являются утверждённым словарём брифа; заменяются на этапах FE-1–FE-5.
 */
export const SCAFFOLD = {
  appTitle: 'AgroPulse',
  appSubtitle: 'Мониторинг вегетации',
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
