import createClient from 'openapi-fetch';

/**
 * HTTP-клиент к Go-бэкенду (frontend-plan §4, corrections §1).
 * baseUrl = VITE_API_URL ?? '' (same-origin: фронт и API отдаёт один Go-контейнер).
 * Никаких запросов к иным хостам, кроме этого baseUrl.
 * Схема paths — рукописный минимум до FE-7; на FE-7 заменяется на сгенерированную
 * openapi-typescript из живого /openapi.json.
 */

export const API_BASE_URL: string = import.meta.env.VITE_API_URL ?? '';

export interface paths {
  '/api/areas': {
    get: { responses: { 200: { content: { 'application/json': unknown } } } };
    post: {
      requestBody?: { content: { 'application/json': unknown } };
      responses: { 201: { content: { 'application/json': unknown } } };
    };
  };
  '/api/areas/{id}': {
    get: {
      responses: {
        200: { content: { 'application/json': unknown } };
        404: { content: { 'application/json': unknown } };
      };
    };
    delete: { responses: { 204: { content: { 'application/json': unknown } } } };
  };
  '/api/areas/{id}/analyses': {
    post: {
      requestBody?: { content: { 'application/json': unknown } };
      responses: {
        202: { content: { 'application/json': unknown } };
        429: { content: { 'application/json': unknown } };
      };
    };
  };
  '/api/areas/{id}/series': {
    get: {
      parameters?: { query?: Record<string, string> };
      responses: { 200: { content: { 'application/json': unknown } } };
    };
  };
  '/api/areas/{id}/events': {
    get: {
      parameters?: { query?: Record<string, string> };
      responses: { 200: { content: { 'application/json': unknown } } };
    };
  };
  '/api/regions/contours': {
    get: {
      parameters?: { query?: Record<string, string> };
      responses: { 200: { content: { 'application/json': unknown } } };
    };
  };
  '/api/jobs/{id}': {
    get: {
      responses: {
        200: { content: { 'application/json': unknown } };
        404: { content: { 'application/json': unknown } };
      };
    };
  };
  '/api/config': {
    get: {
      responses: {
        200: { content: { 'application/json': unknown } };
        404: { content: { 'application/json': unknown } };
      };
    };
  };
}

/**
 * Единая ошибка приложения: любой сбой (RFC 7807, {code,message}, {error},
 * сеть) нормализуется к этому виду. UI работает только с AppError (corrections §1).
 */
export interface AppError {
  /** HTTP-статус; 0 — сетевая ошибка (запрос не дошёл). */
  status: number;
  code: string;
  title: string;
  detail?: string;
  /** Полное тело ошибки (например, limits в 422) — читают адаптеры. */
  extra?: Record<string, unknown>;
}

export function isAppError(value: unknown): value is AppError {
  return (
    typeof value === 'object' &&
    value !== null &&
    'status' in value &&
    'code' in value &&
    'title' in value
  );
}

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.length > 0 ? value : undefined;

/** RFC 7807 / {code,message} / {error} / произвольное тело → AppError. */
export function normalizeError(status: number, body: unknown): AppError {
  const bodyObject =
    typeof body === 'object' && body !== null ? (body as Record<string, unknown>) : null;

  let code = 'unknown';
  let title = 'Ошибка запроса';
  let detail: string | undefined;

  if (!bodyObject) {
    title = `HTTP ${status}`;
  } else if ('title' in bodyObject || 'type' in bodyObject) {
    // RFC 7807: type/title/detail (+code как расширение команды)
    title = asString(bodyObject.title) ?? title;
    detail = asString(bodyObject.detail);
    const typeTail = asString(bodyObject.type)?.split('/').pop();
    code = asString(bodyObject.code) ?? (typeTail && typeTail !== 'problem' ? typeTail : 'problem');
  } else if ('code' in bodyObject) {
    code = String(bodyObject.code);
    title = asString(bodyObject.message) ?? asString(bodyObject.error) ?? code;
    detail = asString(bodyObject.message);
  } else if ('error' in bodyObject) {
    code = 'error';
    title =
      typeof bodyObject.error === 'string'
        ? bodyObject.error
        : (asString(bodyObject.message) ?? 'Ошибка запроса');
    detail = asString(bodyObject.message);
  } else {
    title = JSON.stringify(bodyObject).slice(0, 200);
  }

  // corrections §1: 429 при заполненной очереди — код queue_full, даже если тело минимальное
  if (status === 429 && code === 'unknown') {
    code = 'queue_full';
  }

  return { status, code, title, detail, extra: bodyObject ?? undefined };
}

export function normalizeNetworkError(error: unknown): AppError {
  return {
    status: 0,
    code: 'network',
    title: 'network_error',
    detail: error instanceof Error ? error.message : undefined,
  };
}

export const client = createClient<paths>({
  baseUrl: API_BASE_URL,
  // openapi-fetch фиксирует fetch в момент создания клиента (до старта MSW-перехвата).
  // Отложенный lookup гарантирует, что запросы идут через актуальный globalThis.fetch.
  fetch: (input: Request, init?: RequestInit): Promise<Response> => globalThis.fetch(input, init),
});

type ClientResult<T> = Promise<{ data?: T; error?: unknown; response: Response }>;
type GetPath = Parameters<typeof client.GET>[0];
type PostPath = Parameters<typeof client.POST>[0];
type DeletePath = Parameters<typeof client.DELETE>[0];

async function unwrap<T>(call: () => ClientResult<T>): Promise<T> {
  let result: { data?: T; error?: unknown; response: Response };
  try {
    result = await call();
  } catch (error) {
    // Сетевой сбой (fetch reject): статус 0, код network
    throw normalizeNetworkError(error);
  }
  if (!result.response.ok) {
    throw normalizeError(result.response.status, result.error ?? result.data);
  }
  return result.data as T;
}

const compactQuery = (
  query?: Record<string, string | undefined>,
): Record<string, string> | undefined => {
  if (!query) return undefined;
  const entries = Object.entries(query).filter(
    (entry): entry is [string, string] => entry[1] != null,
  );
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
};

export function apiGet<T>(
  path: GetPath,
  opts?: { path?: Record<string, string>; query?: Record<string, string | undefined> },
): Promise<T> {
  const params = {
    path: opts?.path as never,
    query: compactQuery(opts?.query) as never,
  };
  return unwrap<T>(() => client.GET(path, { params }) as ClientResult<T>);
}

export async function apiPost<T>(
  path: PostPath,
  body?: unknown,
  pathParams?: Record<string, string>,
): Promise<T> {
  const params = { path: pathParams as never };
  return unwrap<T>(() => client.POST(path, { params, body } as never) as ClientResult<T>);
}

export async function apiDelete(
  path: DeletePath,
  pathParams?: Record<string, string>,
): Promise<void> {
  const params = { path: pathParams as never };
  await unwrap<void>(() => client.DELETE(path, { params } as never) as ClientResult<void>);
}
