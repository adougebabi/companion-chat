import { proxy, snapshot, subscribe } from "valtio/vanilla";

type Methods = Record<string, (...args: any[]) => any>;
type Derived<G> = {
  readonly [K in keyof G]: G[K] extends (...args: any[]) => infer R ? R : never;
};
type Definition<S, G, A> = {
  state: () => S;
  getters: G & ThisType<S & Derived<G> & A>;
  actions: A & ThisType<S & Derived<G> & A>;
};

/** Domain actions keep their typed state receiver without owning a UI framework. */
export function defineDomain<
  S extends object,
  G extends Record<string, (state: S) => any>,
  A,
>(definition: Definition<S, G, A>) {
  return definition;
}

export function createDomainStore<
  S extends object,
  G extends Methods,
  A extends object,
>(definition: Definition<S, G, A>) {
  const state = proxy(definition.state()) as S & Derived<G> & A;
  for (const [name, getter] of Object.entries(definition.getters)) {
    Object.defineProperty(state, name, {
      configurable: true,
      get: () => getter.call(state, state),
    });
  }
  for (const [name, action] of Object.entries(definition.actions as Methods)) {
    Object.defineProperty(state, name, {
      configurable: true,
      value: action.bind(state),
    });
  }
  const listen = (listener: () => void) => subscribe(state, listener, true);
  return Object.assign(state, {
    subscribe: listen,
    getSnapshot: () => snapshot(state),
    // Existing imperative domain consumers use this subscription spelling.
    $subscribe: (listener: () => void, _options?: unknown) => listen(listener),
    $reset: () => Object.assign(state, definition.state()),
  });
}
