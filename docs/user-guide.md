# WarmDesk — User Guide

## Contents

1. [Getting Started](#1-getting-started)
2. [The Interface](#2-the-interface)
3. [Projects](#3-projects)
4. [Kanban Board](#4-kanban-board)
5. [Cards](#5-cards)
6. [Topics](#6-topics)
7. [Chats & Direct Messages](#7-chats--direct-messages)
8. [Notifications & @Mentions](#8-notifications--mentions)
9. [Time Reports](#9-time-reports)
10. [User Settings](#10-user-settings)
11. [Search](#11-search)

---

## 1. Getting Started

### Registering

Open your WarmDesk URL in a browser. If public registration is enabled you will
see a **Register** link on the login page. Fill in a username, display name,
email address, and password and click **Register**. You are logged in
immediately.

If registration is not available, ask an administrator to create an account for
you (see the [Admin Guide](admin-guide.md)).

### Logging in

Enter your username and password. WarmDesk issues a short-lived JWT access
token (15 minutes) and a 7-day refresh token that is used silently to keep you
logged in as long as your browser tab is open.

### Session timeout

By default the session expires after **60 minutes of inactivity**. Any
interaction with the page (navigation, clicks, API calls) resets the timer. The
administrator can change this timeout or disable it entirely.

---

## 2. The Interface

### Sidebar

The collapsible sidebar on the left (or right — configurable in User Settings)
contains:

| Section | Contents |
|---------|----------|
| **Starred Projects** | Your pinned projects, collapsed by default |
| **Projects** | All projects you belong to; starred ones appear at the top with a star icon |
| **Online Users** | Users currently connected; click a name to open a direct message |
| **Chats** | Your 8 most recent conversations with unread indicators |

### Header

The header shows the application name on the left and your display name on the
right. Click your name to open User Settings.

### Footer

Displays the application name and version number on the left, and your full
display name on the right.

### Themes

WarmDesk supports **Light**, **Dark**, and **System** (follows your OS
preference) themes. Change the theme in User Settings at any time.

---

## 3. Projects

### Creating a project

Click **+ New Project** in the sidebar or on the dashboard. Give the project a
name, optional description, and colour. Each project gets a short **key prefix**
(e.g. `PRJ`) derived from its name, used in card references like `PRJ-42`. You
can view and reference this prefix on any card detail.

### Project members

Open **Project Settings → Members** to invite team members. Select a user from
the dropdown and choose their role:

| Role | Permissions |
|------|-------------|
| **Viewer** | Read board, cards, topics, and chat. Cannot create or modify anything. |
| **Member** | All viewer permissions plus: create/edit/move cards, post comments, chat. |
| **Admin** | All member permissions plus: manage columns (create, rename, reorder, delete). |
| **Owner** | All admin permissions plus: manage members and webhooks, delete the project. |

Global administrators can do everything regardless of project role.

### Starring a project

Click the star icon next to a project name in the sidebar or on the project
board to pin it to the **Starred Projects** section. Click again to unstar.

### Project settings

The gear icon on the board toolbar (visible to project admins and owners) opens
**Project Settings**, which has tabs for:

- **General** — rename or delete the project
- **Members** — invite, change roles, remove members
- **Labels** — create and manage card labels for this project
- **API Keys** — generate API keys for the Ticket API
- **Webhooks** — set up git platform integrations (see the [API Reference](api.md))

---

## 4. Kanban Board

### Columns

Each project has one or more columns (e.g. Backlog, In Progress, Done).
Project admins can:

- **Rename** a column by clicking its title
- **Add** a column with the **+ Add Column** button
- **Reorder** columns by dragging the column header
- **Delete** an empty column using the trash icon that appears when the column
  has no cards

### Cards in columns

Cards are shown as tiles within each column. Each tile shows the card title,
labels, priority indicator, due date, assignee avatars, and a checklist
completion counter if the card has checklist items.

### Sorting cards

Use the sort controls at the top of any column to order cards by:
- **Date** (creation date) — ascending or descending
- **Assignee** — alphabetical by display name
- **Priority** — critical → high → medium → low → none

### Moving cards

Drag a card to a different column or a different position within the same
column. All connected users see the move reflected instantly.

---

## 5. Cards

### Creating a card

Click **+ Add Card** at the bottom of any column. Enter a title and press Enter
or click **Add**.

### Card reference

Every card has a unique reference in the format `PREFIX-NUMBER` (e.g. `PRJ-42`).
This reference appears as a badge at the top of the card detail. Use it in
commit messages and pull request titles to automatically link git events to the
card (see [Git Integration](api.md#git-platform-webhooks)).

### Card detail

Click a card to open its detail panel. The panel is resizable. Fields:

| Field | Notes |
|-------|-------|
| **Title** | Plain text |
| **Description** | Markdown editor with toolbar, emoji picker, and @mention autocomplete |
| **Priority** | None / Low / Medium / High / Critical |
| **Due date** | Displayed in your configured date format. Type a date directly or click the calendar icon (📅) to open the native date picker. Clear the field to remove the due date. |
| **Time Spent** | Hours and minutes; contributes to time reports |
| **Assignee** | Single primary assignee (legacy) |
| **Assignees** | Multiple assignees — click a name to toggle |
| **Labels** | Click a label chip to toggle; labels are project-specific |
| **Tags** | Free-form hashtags; type and press Enter or comma |
| **Watchers** | Subscribe to receive mention notifications on this card |
| **Attachments** | Upload files by clicking or dragging; images display inline |
| **Checklist** | Add, check off, edit, and remove items; a progress bar shows completion % |
| **Git Links** | Commits, pull requests, and issues linked via webhooks (auto-populated) |
| **Comments** | Markdown, @mentions, and reply quoting |
| **Column history** | Log of every column transition |

### Writing in the card editor

The description and comment editors use **EasyMDE**, which supports GitHub
Flavored Markdown. Useful shortcuts:

| Action | Shortcut |
|--------|----------|
| Bold | Ctrl+B |
| Italic | Ctrl+I |
| Heading | Ctrl+H |
| Unordered list | Ctrl+L |
| Preview | Ctrl+P |
| Insert emoji | Click 😊 in the toolbar |
| @mention | Type `@` followed by a username |

#### Emoticon shortcuts

The editor automatically replaces common text emoticons with emoji as you type:

| Type | Gets replaced with |
|------|--------------------|
| `:-)` | 😊 |
| `:-)` | 😊 |
| `:-D` | 😄 |
| `;-)` | 😉 |
| `:-(` | 😞 |
| `>:-(` | 😠 |
| `:'(` | 😢 |
| `O:-)` | 😇 |
| `<3` | ❤️ |
| `:+1:` | 👍 |
| `:-P` | 😛 |
| `B-)` | 😎 |
| `8-)` | 😎 |
| `:-O` | 😮 |
| `:-*` | 😘 |

### Checklist

In the card detail scroll to **Checklist** and type an item, then press Enter
or click **Add Item**. Check items off as they are completed. A progress bar at
the top of the section shows how many items are done. Edit an item by clicking
the pencil icon; delete it with the ×.

### Attachments

Drag files onto the upload zone or click it to open a file picker. Multiple
files can be uploaded at once. Images are displayed inline with a link to the
full-size version; other files appear as download links with their file name and
size.

### Git Links

When a commit message or pull request / issue title contains a card reference
(e.g. `PRJ-42`), WarmDesk creates a link automatically. Each link shows:

- **Platform badge** — GitHub, GitLab, Gitea, or Forgejo
- **Type** — Commit, Pull Request, or Issue
- **Short reference** — first 7 characters of the SHA for commits, or `#number`
  for PRs and issues
- **Title** — the commit message first line or PR / issue title
- **Status badge** — Open (green), Closed (red), or Merged (purple)

Click any row to open the original event in your git platform.

### Comments

Type in the **Add Comment** editor at the bottom of the card detail. Comments
support full Markdown. To reply to a comment, click the **Reply** button below
it — the comment is quoted automatically.

Use `@username` to mention a project member. If they are online they receive an
instant popup notification; offline they receive an email.

### Saving

Click **Save** in the card detail footer to persist changes to the title,
description, priority, due date, assignees, and time spent. Labels, tags,
watchers, and assignees are saved immediately when toggled — no need to click
Save.

---

## 6. Topics

Topics are threaded project-level discussions — useful for decisions, planning,
or announcements that do not belong on a specific card.

### Creating a topic

Navigate to a project and click **Topics** in the navigation, then **New
Topic**. Give it a title and body (Markdown supported). @mentions are supported
and send notifications.

### Replying

Open a topic and write your reply in the box at the bottom. Replies support
Markdown and @mentions.

### Editing and deleting

The author (and project owners / admins) can edit or delete a topic or any of
its replies by clicking the ✏ or × icons.

---

## 7. Chats & Direct Messages

### Project chat

Each project has a dedicated **Chat** page accessible from the project
navigation. Messages support Markdown, emoji (picker available), @mentions,
file attachments, and emoji reactions. Click any emoji reaction to toggle your
own reaction.

### Direct messages

Click a user's name in the **Online Users** sidebar section, or open the
**Chats** page and start a new conversation. Direct messages are one-on-one
by default.

### Group chats

On the **Chats** page, click **New Conversation**, switch to the **Group** tab,
and add multiple members. Give the group a name. You can later:

- **Add members** by clicking the + icon in the chat header
- **Remove members** by clicking × next to their name in the header
- **Change the group avatar** by clicking the group icon in the header

### Teams tab

In **Chats → New Conversation → Teams**, you can see all projects you belong
to. Clicking a project pre-fills all its members and the project name as a
group conversation starter — handy for creating a team chat in one click.

### Unread indicators

A pulsing red dot appears next to a conversation in the **Chats** sidebar
section when there are messages you have not seen. The dot clears as soon as
you open the conversation. The main navigation item also shows an indicator when
any conversation has unread messages.

### Emoji reactions

Hover over any message and click the 😊 button to open the emoji picker, then
click an emoji to react. Click an existing reaction badge to toggle your own
reaction on or off.

---

## 8. Notifications & @Mentions

### Real-time mention popups

When another user types `@yourname` in a chat message, card comment, or topic
reply and sends it while you are online, a purple notification popup appears in
the corner of the screen. It shows:

- Who mentioned you
- The context (project chat / card comment / direct message)
- A two-line preview of the message

The popup dismisses automatically after 6 seconds.

### Email notifications

If you are **offline** (no open tab) when someone mentions you, WarmDesk sends
an email to your registered address — provided the administrator has configured
SMTP. The email contains the sender, context, and message preview.

---

## 9. Time Reports

### Generating a report

Open **Reports** from the main navigation. Choose:

| Filter | Options |
|--------|---------|
| **Period** | All time / This year / This month / This week (ISO week) |
| **Project** | All projects or a specific one |
| **Assignees** | All or one or more specific users |

The report shows a table of cards with time logged, grouped by project. Totals
are shown in H:MM format.

### Exporting

- **PDF** — click **Export PDF**. The page prints using `@media print` styles
  that hide the sidebar, header, and footer, showing only the report table with
  the company name and logo (if configured).
- **Excel (XLSX)** — click **Export Excel** to download a `.xlsx` file with the
  same data ready for further analysis.

---

## 10. User Settings

Open User Settings by clicking your display name in the header.

| Setting | Notes |
|---------|-------|
| **Display name** | Shown throughout the UI and in reports |
| **Email** | Used for notifications and Gravatar |
| **Avatar** | Upload an image or use your Gravatar (via email address) |
| **Language** | English (en), Dutch (nl), German (de), French (fr), Spanish (es) |
| **Theme** | Light / Dark / System |
| **Date format** | e.g. `YYYY-MM-DD` (ISO default), `DD/MM/YYYY`, `MM/DD/YYYY` |
| **Time format** | 24-hour or 12-hour |
| **Timezone** | UTC by default; affects date display throughout the UI |
| **Font** | Interface font (system, inter, roboto, etc.) |
| **Font size** | Small / Medium / Large |
| **Sidebar position** | Left (default) or right |
| **Change password** | Enter current password and a new one |

---

## 11. Search

The global search bar (magnifying glass icon in the header) searches across:

- Card titles and descriptions
- Card comments
- Project names

Results are grouped by type and clicking one navigates directly to it.
