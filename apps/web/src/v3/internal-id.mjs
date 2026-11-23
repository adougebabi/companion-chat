const internalIdPattern = /^(?:world_item|world|place|req|actor|fluctlight|res|mem|memory|rel|relationship|claim|self)_[a-z0-9][a-z0-9_-]*$/i;

export function isInternalId(value) {
  return typeof value === 'string' && internalIdPattern.test(value.trim());
}
