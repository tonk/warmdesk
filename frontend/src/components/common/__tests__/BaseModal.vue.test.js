import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/en.json'
import BaseModal from '../BaseModal.vue'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })
let wrapper

const mountModal = () => {
  wrapper = mount(BaseModal, {
    props: { title: 'Time entry' },
    slots: { default: '<input id="desc" />' },
    global: { plugins: [i18n] },
    attachTo: document.body,
  })
  return {
    backdrop: document.querySelector('.modal-backdrop'),
    input: document.querySelector('#desc'),
  }
}

afterEach(() => wrapper?.unmount())

const fire = (el, type) => el.dispatchEvent(new MouseEvent(type, { bubbles: true }))

describe('BaseModal backdrop', () => {
  it('closes on a click on the backdrop', () => {
    const { backdrop } = mountModal()
    fire(backdrop, 'mousedown')
    fire(backdrop, 'click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('stays open when a press in an input is released over the backdrop', () => {
    const { backdrop, input } = mountModal()
    fire(input, 'mousedown')
    // the browser sends the click to the common ancestor: the backdrop
    fire(backdrop, 'click')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
