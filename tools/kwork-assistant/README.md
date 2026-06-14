# Kwork Assistant

Autopilot for a Kwork seller account.

Normal use is one command from the repo root:

```powershell
D:\claude\vk_bot\start-kwork-assistant.bat
```

The batch file:

- enters `tools\kwork-assistant`;
- creates `.env` from `.env.example` if missing;
- installs `playwright-core` if `node_modules` is missing;
- starts Chromium through Playwright;
- monitors Kwork inbox;
- sends replies automatically when a new request is detected.

On the first run, autopilot marks currently visible threads as old by default. This prevents it from replying to every old dialog already on the page. New threads after that are processed and answered.

## What To Fill

The first run creates these private files:

- `data/profile.md` - services, prices, deadlines, tone, stop topics.
- `data/examples.md` - examples of good incoming briefs and your successful replies.

They are ignored by Git.

## Important Settings

Edit `.env` only when needed:

```env
CHROMIUM_EXECUTABLE_PATH=C:\Users\GigaChat\AppData\Local\ms-playwright\chromium-1208\chrome-win64\chrome.exe
KWORK_USER_DATA_DIR=.\profile
KWORK_INBOX_URL=https://kwork.ru/inbox
KWORK_AUTO_SEND=true
KWORK_AUTOPRIME_ON_FIRST_RUN=true
AI_ENGINE=pi
PI_COMMAND=pi
PI_ARGS=--mode rpc --no-session
```

If Pi is installed but the `pi` command is not in PATH, set `PI_COMMAND` to the real executable path or wrapper command.

## Service Commands

These are only for debugging:

```powershell
npm.cmd run login
npm.cmd run prime
npm.cmd run once
npm.cmd run monitor
npm.cmd run reply -- data\drafts\file.md --send
```

