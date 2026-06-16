// postChangeRedirect decide para onde a ChangePasswordPage navega após trocar a
// senha com sucesso.
// - Fluxo FORÇADO (1º login, mustChangePassword): mantém o onboarding — admin sem
//   câmeras vai para o cadastro de câmera; senão, home.
// - Fluxo MANUAL (botão "Alterar senha"): volta SEMPRE para a tela de origem
//   (`from`), com fallback para a home quando não há origem.
export function postChangeRedirect(opts: {
  forced: boolean
  from?: string | null
  adminWithNoCameras: boolean
}): string {
  if (opts.forced) return opts.adminWithNoCameras ? '/settings/cameras/new' : '/'
  return opts.from || '/'
}
