// Per 09-RESEARCH §Code Examples §Manual MockWebSocket; D-37 D-04 Phase 7 pattern.
// jsdom does NOT provide WebSocket global (Pitfall 9). This is OPT-IN: tests that need WS must
// install globalThis.WebSocket = MockWebSocket in beforeEach + restore in afterEach.

export class MockWebSocket {
  static instances: MockWebSocket[] = [];
  static OPEN = 1 as const;
  static CLOSED = 3 as const;

  readyState: 0 | 1 | 3 = 0;
  url: string;
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  // Test-controlled lifecycle (not part of real WebSocket API)
  open() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  emit(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent);
  }

  closeFromServer(code = 1006) {
    this.readyState = 3;
    this.onclose?.({ code, reason: '', wasClean: false } as CloseEvent);
  }

  // Real WebSocket API
  close(code = 1000, _reason?: string) {
    this.readyState = 3;
    this.onclose?.({ code, reason: '', wasClean: true } as CloseEvent);
  }

  send(_data: string) {
    // no-op for tests
  }

  static reset() {
    MockWebSocket.instances = [];
  }
}
