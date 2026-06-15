import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import ConfirmDialog from './ConfirmDialog'

afterEach(cleanup)

function renderDialog(props: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  return render(
    <ConfirmDialog
      open
      title="Excluir"
      message="Tem certeza?"
      onConfirm={vi.fn()}
      onCancel={vi.fn()}
      {...props}
    />,
  )
}

describe('ConfirmDialog', () => {
  it('os botões têm ids estáveis', () => {
    renderDialog()
    expect(document.getElementById('confirm-dialog-confirm')).toBeTruthy()
    expect(document.getElementById('confirm-dialog-cancel')).toBeTruthy()
  })

  it('danger usa o variant destructive no confirmar', () => {
    renderDialog({ danger: true })
    expect(document.getElementById('confirm-dialog-confirm')!.className).toContain('bg-destructive')
  })

  it('sem danger usa o variant default (primary) no confirmar', () => {
    renderDialog({ danger: false })
    expect(document.getElementById('confirm-dialog-confirm')!.className).toContain('bg-primary')
  })

  it('o cancelar usa o variant outline', () => {
    renderDialog()
    expect(document.getElementById('confirm-dialog-cancel')!.className).toContain('border-input')
  })

  it('dispara onConfirm e onCancel preservando os handlers', () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    renderDialog({ onConfirm, onCancel })
    fireEvent.click(document.getElementById('confirm-dialog-confirm')!)
    fireEvent.click(document.getElementById('confirm-dialog-cancel')!)
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('preserva os textos customizados dos botões', () => {
    renderDialog({ confirmLabel: 'Apagar tudo', cancelLabel: 'Voltar' })
    expect(document.getElementById('confirm-dialog-confirm')!.textContent).toBe('Apagar tudo')
    expect(document.getElementById('confirm-dialog-cancel')!.textContent).toBe('Voltar')
  })
})
