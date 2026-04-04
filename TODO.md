- Bugfix:
  On Fedora 43 this setting must be done to not show the blank screen:
  LD_PRELOAD=/usr/lib64/libwayland-client.so
  Client was built on Ubuntu 24.04. Could be that this library is
  LD_PRELOAD=/usr/lib/libwayland-client.so on Ubuntu
  Fix this, once and for all
- In the client, an issue created in Gitea is displayed on the card, but
  when clicked, nothing happens. The link should be opened in a browser.
  This works in the browser version.
- Investigate:
  When starting the Windows client it takes rather long before
  it responds… for userid/password entry

- Nice to have:
  * Change projects to add an extra layer
    so that we can have "Customer / Projects"
