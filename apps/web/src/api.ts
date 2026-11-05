export interface SyntheticEvent {
  event_id: string
  fixture_id: string
  source: 'injected' | 'sms_broadcast'
  received_at_ms: number
  subscription_id: number | null
}

export interface EventPage {
  installation_id: string
  events: SyntheticEvent[]
  total: number
  next_after: string | null
}

export const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/

function validEvent(event: SyntheticEvent): boolean {
  return event !== null && typeof event === 'object' &&
    typeof event.event_id === 'string' && uuidPattern.test(event.event_id) &&
    event.fixture_id === event.event_id && ['injected', 'sms_broadcast'].includes(event.source) &&
    Number.isSafeInteger(event.received_at_ms) && event.received_at_ms > 0 && event.received_at_ms < 4102444800000 &&
    (event.subscription_id === null || (Number.isInteger(event.subscription_id) && event.subscription_id >= 0 && event.subscription_id <= 2147483647))
}

export async function readEvents(installationId: string, after: string | null): Promise<EventPage> {
  const query = new URLSearchParams({ installation_id: installationId, limit: '25' })
  if (after) query.set('after', after)
  const response = await fetch(`/poc/v1/events?${query}`, {
    cache: 'no-store', credentials: 'omit', signal: AbortSignal.timeout(5000),
  })
  if (!response.ok) throw new Error('read_failed')
  const page: EventPage = await response.json()
  if (page === null || page.installation_id !== installationId || !Array.isArray(page.events) ||
      page.events.length > 25 || !page.events.every(validEvent) ||
      new Set(page.events.map(event => event.event_id)).size !== page.events.length ||
      !Number.isSafeInteger(page.total) || page.total < 0 ||
      !(page.next_after === null || (typeof page.next_after === 'string' && uuidPattern.test(page.next_after)))) {
    throw new Error('invalid_response')
  }
  let previous = after ?? ''
  for (const event of page.events) {
    if (event.event_id <= previous) throw new Error('invalid_order')
    previous = event.event_id
  }
  if (page.events.length > page.total || (page.next_after !== null && page.next_after !== page.events.at(-1)?.event_id)) {
    throw new Error('invalid_page')
  }
  return page
}
