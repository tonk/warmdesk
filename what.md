Create an application that has all these features and requirements

- Do not use Docker or Podman
- Written in Golang, minumum version 1.25
- Webbased
- Kanban board
- Multi user
- Multi project
- Multi language support
  * English
  * Dutch
  * German
  * French
  * Spanish
- Database support for
  * SQLlite
  * MySQL
  * PostgreSQL
- Database configuration through config file
- Allow another config file as an option on the command line
- Role based access control
- Internal chat function
- Group chat between members
- Allow chat to be scaled horizontally
- All text editors must support Github markdown
- Allow for dark, light and follow-system interface
- Add settings per user
  * First name
  * Last name
  * Email address
  * Avatar, with Gravatar support
  * Change password
  * Date / time format
    ** Use IЅO timeformat as the default
  * Timezone
    ** Use UTC as the default
  * Interface font and font size
- Allow Admin settings for
  * For all users
    ** Change all user settings
    ** Change all project settings
  * Create new users
  * Disable users
  * Delete users
  * Switch the register on the login page on/off
    ** Remove "Don't have an account? Register" from the login page if
       the setting is off
  * Global system settings, can be overruled by the user
    ** Date / time format
       ** Use IЅO timeformat as the default
    * * Timezone
      ** Use UTC as the default
    ** Interface color, dark, light or follow-system interface
    ** Interface font and font size
    ** Default language setting, English by default
  * Add, change and remove projects
- Keep track of last login
- Keep track of last user settings change
- Enable a sidebar
  * Collapsible list of starred projects
  * Collapsible list of all projects, starred project marked and at the top
  * Collapsible list of all users, in chat users marked and at the top
  * Make the users clickable and when clicked, open an direct chat box to them
- Allow sidebar to be moved to the left or right
- In the projects
  * Allow the column names to be edited
  * Allow the columns to be moved
  * Allow project to be starred for the sidebar
  * In the "Invite members" box, give a pull down with all active users
- When a new project is created, automatically create a default column
  called "Inbox"
- In the ticket overview, place the users avatar at the top right of the
  box
- In the tickets
  * Keep history of column changes
  * In the "Edit Card" box
    ** Allow the box to be resized horizontal and vertical
    ** Add "Save" button
    ** Start the date picker with the current date
    ** "Due date" field must follow user date / time settings
    ** "Due date" must be saved with the card
- Enable an API to control all tickets
  * Add
  * Comment
  * Move to other column
- Add a footer to the UI
  * Application name and version number
    ** Align left
  * Logged users full name
    ** Align right
- In the tickets
  * Allow multiple assignees per card
  * Allow watching a card to receive notifications
  * Allow sorting cards within a column by due date, assignee or priority
  * Add a checklist to a card
  * Add a "Time Spent" field (hours and minutes) to log effort on a card
- Add topics (threaded discussions) per project
  * Create, edit and delete topics
  * Reply to topics with markdown support
- Add a viewer role to role based access control
  * Viewers can read but not create or modify content
- Favourite people
  * Mark users as favourites for quick access
- Direct message notifications
  * Notify users of new direct messages
- Group direct messages
  * Start a group chat between multiple users
- Admin can assign users to projects directly
- Allow admin to reset user passwords
- Build a desktop app using Tauri
  * Distribute as AppImage (Linux), DMG (macOS) and installer (Windows)
- SMTP email configuration through the admin web interface
  * Username and password are optional (support auth-less relay servers)
  * Settings take effect without a server restart
- Time reports
  * Generate a time report filtered by period (all time, year, month, ISO week)
    and optionally by project
  * Export to PDF (print-optimised layout)
  * Export to Excel (XLSX)
  * Report header shows configurable company name and logo
- Company branding
  * Admin can set a company name and logo (URL or uploaded image)
  * Used in report headers
- Demo seed tool
  * A standalone `coworker-seed` binary included in the distribution
  * Populates the database with demo users (admin, members, viewer), projects,
    cards, checklists, comments, time entries, and discussion topics
  * Idempotent — safe to run multiple times; supports --reset flag

- Add README.md with explanation of Coworker
- Add systemd example service files
- Add Nginx example configuration file
- Add Apache example configuration file
- Add a decent .gitignore file
- Build production version
- Create installation manual
- Create distribution package containing everything needed
- Create a Github actions file to build a new release when a new tag is
  pushed in the main branch
- Add CLAUDE.md developer guide for AI-assisted development
- Show version number on the login page
- Frontend version number must follow the git tag automatically (no manual updates)
- All projects visible to admins in the sidebar
- PDF time report must print only the report content (no sidebar, header, or footer)
- Time in reports displayed as H:MM (hours unbounded, minutes zero-padded)
- Default initial project column renamed from "Inbox" to "Backlog"
- Configurable initial columns per new project (admin setting, one name per line)
- Delete empty columns from the board view
- Assignee filter on time reports (select one, multiple, or all assignees)
- Direct message history loads from database when opening a conversation
- Remove a member from a group chat
- Demo conversations in the seed tool (4 DMs + 1 group chat with realistic message history)
- Screenshots of all main views, referenced in the README
- Project admin role (column management) separate from member and owner
- Board toolbar shows project name; settings gear visible only to admins
- Upload an avatar image for group chats
- Auto-delete empty group chat when last non-creator member is removed
- Start a group chat from a project team in Direct Messages (Teams tab)
- More demo users in seed (Priya, James, Elena, Raj) with project admin roles
- Persistent system admin (tonk) in seed, not affected by --reset
- Emoji picker in all chat inputs and card editors (EasyMDE toolbar button)
- @mention autocomplete in all chat inputs and card editors with dropdown navigation
- Real-time popup notifications for @mentions when the user is online; email for offline
- Chats sidebar section with per-conversation unread indicators
- SMTP test email in admin settings
- "Direct Messages" renamed to "Chats" throughout; /messages redirects to /chats
- Team Chat removed from project board page
- Card comments now trigger @mention notifications (email + real-time WS)
