# -*- coding: utf-8 -*-
# GNU General Public License v2.0+ (see COPYING or https://www.gnu.org/licenses/gpl-2.0.txt)
from __future__ import absolute_import, division, print_function
__metaclass__ = type

"""
Name-to-ID resolvers for the ansilabnl.warmdesk collection.

Modules that need a numeric ID (e.g. customer_id for a contract) call these
helpers rather than doing the lookup inline.  All resolvers raise
WarmDeskAPIError(404, …) when the named resource cannot be found.

Resolvers are intentionally simple: they fetch the relevant list and scan it.
WarmDesk has no server-side search for most resources, and the lists are
small enough that a linear scan is fine.
"""

from ansible_collections.ansilabnl.warmdesk.plugins.module_utils.warmdesk_api import (
    WarmDeskAPIError,
)


def resolve_user_id(client, username, project_slug=None):
    """Return the numeric ID for *username* (or email address).

    Tries GET /users first.  When that returns 403 (project-scoped API key)
    and *project_slug* is supplied, falls back to GET /projects/{slug}/members
    which is always accessible to project-scoped keys.  Both endpoints are
    searched by username and by email address so either form works.
    """
    def _match(users, key):
        for u in users:
            if u.get('username') == key or u.get('email') == key:
                return u['id']
        return None

    try:
        users = client.get('/users')
        uid = _match(users, username)
        if uid is not None:
            return uid
    except WarmDeskAPIError as exc:
        if exc.status != 403 or project_slug is None:
            raise

    # Fall back to the project members list (works with project-scoped keys)
    if project_slug is None:
        raise WarmDeskAPIError(404, 'User not found: %s' % username)
    members = client.get('/projects/%s/members' % project_slug)
    uid = _match([m['user'] for m in members if 'user' in m], username)
    if uid is not None:
        return uid
    raise WarmDeskAPIError(404, 'User "%s" not found in project %s' % (username, project_slug))


def resolve_customer_id(client, name):
    """Return the numeric ID of the customer whose name equals *name*.

    Includes hidden customers (GET /customers excludes them by default) so a
    customer hidden via the admin UI still resolves instead of looking
    not-found and triggering a duplicate create.
    """
    customers = client.get('/customers', params={'include_hidden': 'true'})
    for c in customers:
        if c['name'] == name:
            return c['id']
    raise WarmDeskAPIError(404, 'Customer not found: %s' % name)


def resolve_contract_id(client, customer_id, name):
    """Return the numeric ID of the contract with *name* under *customer_id*."""
    contracts = client.get('/customers/%d/contracts' % customer_id)
    for c in contracts:
        if c['name'] == name:
            return c['id']
    raise WarmDeskAPIError(404, 'Contract "%s" not found for customer %d' % (name, customer_id))


def resolve_column_id(client, project_slug, column_name):
    """Return the numeric ID of the column named *column_name* in *project_slug*."""
    columns = client.get('/projects/%s/columns' % project_slug)
    for col in columns:
        if col['name'] == column_name:
            return col['id']
    raise WarmDeskAPIError(404, 'Column "%s" not found in project %s' % (column_name, project_slug))


def resolve_label_id(client, project_slug, label_name):
    """Return the numeric ID of the label named *label_name* in *project_slug*."""
    labels = client.get('/projects/%s/labels' % project_slug)
    for lbl in labels:
        if lbl['name'] == label_name:
            return lbl['id']
    raise WarmDeskAPIError(404, 'Label "%s" not found in project %s' % (label_name, project_slug))


def resolve_group_id(client, name):
    """Return the numeric ID of the group whose name equals *name*.

    Requires admin credentials (groups are admin-only).
    """
    groups = client.get('/admin/groups')
    for g in groups:
        if g.get('name') == name:
            return g['id']
    raise WarmDeskAPIError(404, 'Group not found: %s' % name)


def resolve_card_ref(client, card_ref):
    """Resolve a card reference like 'EDA-42' anywhere on the server.

    Uses GET /cards/resolve/{ref}, which also follows the old reference of a
    card that was moved to another project (the server keeps it as an alias).
    Returns a dict with C(id), C(card_number), C(key_prefix) and
    C(project_slug) of the card's *current* location, or None when the
    reference is unknown or not visible to the caller. Project-scoped API keys
    cannot use this endpoint (it has no project in its path); for them this
    also returns None.
    """
    try:
        return client.get('/cards/resolve/%s' % card_ref)
    except WarmDeskAPIError as exc:
        if exc.status in (403, 404):
            return None
        raise


def find_card_by_number(client, project_slug, card_ref, follow_moves=False):
    """Return a card dict given a card reference like 'EDA-42'.

    Iterates all columns in the project to locate the card.  Returns None if
    not found (so callers can decide whether to fail or treat it as absent).

    With *follow_moves*, a reference that isn't a live card of the project is
    also tried as the old reference of a card that was moved *into* this
    project from another one (see resolve_card_ref).
    """
    project = client.get('/projects/%s' % project_slug)
    for col in project.get('columns', []):
        for card in col.get('cards', []):
            ref = '%s-%d' % (project.get('key_prefix', ''), card['card_number'])
            if ref == card_ref:
                return client.get('/projects/%s/cards/%d' % (project_slug, card['id']))
    if follow_moves:
        resolved = resolve_card_ref(client, card_ref)
        if resolved and resolved.get('project_slug') == project_slug:
            return client.get('/projects/%s/cards/%d' % (project_slug, resolved['id']))
    return None
