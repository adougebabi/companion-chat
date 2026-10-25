// Applicability is presentation-only. Never erase an explicit stored value.
export function bodyFieldVisible(key, value, identity = {}) {
  if (value === 'not_applicable' || value?.status === 'not_applicable') return false;
  if (key !== 'chest_cup' || !['male', '男', '男性'].includes(identity.gender)) return true;
  const actual = value && typeof value === 'object' ? value.value : value;
  return actual != null && actual !== '' && actual !== 'unknown';
}
