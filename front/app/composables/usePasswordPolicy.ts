export function usePasswordPolicy() {
  function validatePassword(pw: string): string {
    if (pw.length < 8) return 'Password must be at least 8 characters.'
    if (!/[A-Z]/.test(pw)) return 'Password must contain at least one uppercase letter.'
    if (!/[a-z]/.test(pw)) return 'Password must contain at least one lowercase letter.'
    if (!/[0-9]/.test(pw)) return 'Password must contain at least one digit.'
    if (!/[^A-Za-z0-9]/.test(pw)) return 'Password must contain at least one special character.'
    return ''
  }

  return { validatePassword }
}
