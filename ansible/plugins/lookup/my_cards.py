# -*- coding: utf-8 -*-
# GNU General Public License v2.0+ (see COPYING or https://www.gnu.org/licenses/gpl-2.0.txt)
from __future__ import absolute_import, division, print_function
__metaclass__ = type

DOCUMENTATION = r"""
name: my_cards
short_description: List all open WarmDesk cards assigned to the authenticated user
version_added: "0.6.6"
author: "Ton Kersten (@tonk)"
description:
  - Returns every open card assigned to the authenticated user across all
    projects, via C(GET /api/v1/my-cards) — the same list as the dashboard's
    I(My cards) view.
  - A card counts as assigned when the user is its main assignee or one of
    its extra assignees.
  - Only cards in open projects the user is a member of are returned (global
    admins see all projects). Closed cards are never returned.
  - Results are sorted by due date (soonest first, cards without a due date
    last), then by project name and card number.
notes:
  - The cards are those of the user the credentials belong to — for an API key,
    the key's owner.
  - Requires WarmDesk server v0.43.0 or newer.
  - Project-scoped API keys are rejected, because the endpoint spans projects.
  - Use C(query()) or C(wantlist=true) to always receive a list, also when only
    one card is assigned.

options:
  _terms:
    description:
      - Not used; the lookup takes no terms.
    required: false

  project:
    description:
      - Only return cards of the project with this slug.
    type: str

  url:
    description:
      - Base URL of the WarmDesk instance.
    type: str
    required: true
    env:
      - name: WARMDESK_URL

  token:
    description:
      - Pre-obtained JWT access token.
    type: str
    no_log: true
    env:
      - name: WARMDESK_TOKEN

  username:
    description:
      - WarmDesk login name for password-based authentication.
    type: str
    env:
      - name: WARMDESK_USERNAME

  password:
    description:
      - WarmDesk password for password-based authentication.
    type: str
    no_log: true
    env:
      - name: WARMDESK_PASSWORD

  api_key:
    description:
      - WarmDesk API key (C(X-API-Key) header).
    type: str
    no_log: true
    env:
      - name: WARMDESK_API_KEY

  validate_certs:
    description:
      - Whether to validate TLS certificates.
    type: bool
    default: true
"""

EXAMPLES = r"""
- name: Fetch all my open cards
  ansible.builtin.set_fact:
    my_cards: "{{ query('ansilabnl.warmdesk.my_cards',
                        url='https://warmdesk.example.com',
                        api_key=vault_api_key) }}"

- name: Show them
  ansible.builtin.debug:
    msg: "{{ item.card_ref }} [{{ item.column_name }}] {{ item.title }} (due {{ item.due_date | default('-', true) }})"
  loop: "{{ my_cards }}"

- name: Count my overdue cards
  ansible.builtin.debug:
    msg: >-
      {{ my_cards | selectattr('due_date')
                  | selectattr('due_date', '<', now(utc=true).strftime('%Y-%m-%d'))
                  | list | length }} overdue

- name: Only the cards in one project
  ansible.builtin.set_fact:
    platform_cards: "{{ query('ansilabnl.warmdesk.my_cards', project='platform',
                              url=warmdesk_url, api_key=warmdesk_key) }}"
"""

RETURN = r"""
_list:
  description:
    - One dict per open card assigned to the user, sorted by due date.
  type: list
  elements: dict
  contains:
    id:
      description: Numeric card ID.
      type: int
      sample: 123
    card_number:
      description: Sequential card number within the project.
      type: int
      sample: 42
    card_ref:
      description: Full card reference string (key_prefix + card_number).
      type: str
      sample: EDA-42
    title:
      description: Card title.
      type: str
      sample: Implement OAuth2 login
    priority:
      description: Card priority (C(none), C(low), C(medium), C(high), C(critical)).
      type: str
      sample: high
    due_date:
      description: ISO-8601 due date, or null when none is set.
      type: str
      sample: "2026-11-10T00:00:00Z"
    column_name:
      description: Name of the column the card is in.
      type: str
      sample: In Progress
    updated_at:
      description: ISO-8601 timestamp of the last change.
      type: str
      sample: "2026-11-03T09:12:00Z"
    project:
      description: The card's project.
      type: dict
      contains:
        id:
          description: Numeric project ID.
          type: int
        name:
          description: Project name.
          type: str
        slug:
          description: Project slug.
          type: str
        key_prefix:
          description: Project card key prefix.
          type: str
        color:
          description: Project colour.
          type: str
        customer_id:
          description: ID of the project's customer, or null.
          type: int
        customer_name:
          description: Name of the project's customer (empty when none).
          type: str
"""

from ansible.errors import AnsibleError
from ansible.plugins.lookup import LookupBase
from ansible_collections.ansilabnl.warmdesk.plugins.module_utils.warmdesk_api import (
    WarmDeskClient,
    WarmDeskAPIError,
)


class LookupModule(LookupBase):

    def run(self, terms, variables=None, **kwargs):
        self.set_options(var_options=variables, direct=kwargs)

        url = kwargs.get('url') or ''
        token = kwargs.get('token')
        username = kwargs.get('username')
        password = kwargs.get('password')
        api_key = kwargs.get('api_key')
        validate_certs = kwargs.get('validate_certs', True)
        project_slug = kwargs.get('project') or ''

        if not url:
            raise AnsibleError(
                'my_cards lookup requires the "url" parameter '
                '(or WARMDESK_URL environment variable).'
            )

        if not (api_key or token or (username and password)):
            raise AnsibleError(
                'my_cards lookup requires authentication: '
                'provide api_key, token, or username+password.'
            )

        client = WarmDeskClient(
            url=url,
            username=username,
            password=password,
            token=token,
            api_key=api_key,
            validate_certs=validate_certs,
        )

        try:
            cards = client.get('/my-cards') or []
        except WarmDeskAPIError as e:
            if e.status == 404:
                raise AnsibleError(
                    'my_cards lookup: GET /api/v1/my-cards not found — '
                    'the server must be WarmDesk v0.43.0 or newer.'
                )
            raise AnsibleError(
                'WarmDesk API error %d listing my cards: %s' % (e.status, e.message)
            )

        results = []
        for card in cards:
            project = card.get('project') or {}
            if project_slug and project.get('slug') != project_slug:
                continue
            card['card_ref'] = '%s-%s' % (project.get('key_prefix', ''), card.get('card_number'))
            results.append(card)
        return results
