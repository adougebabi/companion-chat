import { setBlockTracking } from "vue";
/**
 * The supplied compiler's block lists describe its original native DOM tree.
 * Component extraction changes those boundaries. Use Vue's ordinary patching
 * for these retained render bodies instead of reusing stale compiler lists.
 */
export function unoptimizedRenderer(factory) {
  return (bindings) => {
    const render = factory(bindings);
    return (...args) => {
      setBlockTracking(-1);
      try {
        return render(...args);
      } finally {
        setBlockTracking(1);
      }
    };
  };
}
