# Homeport Mobile App Plan

A native mobile companion app for Homeport, focused on terminal access and Claude Code integration.

## Vision

The app enables "set a task and walk away" workflows:
1. Start Claude Code on a task from desktop/web
2. Get push notifications when Claude needs approval
3. Approve/deny from your phone
4. Preview ports Claude opened
5. Reconnect to terminal sessions

## Core Features (Priority Order)

### 1. Terminal Sessions
**Goal:** View and reconnect to running terminal sessions

**Current State:**
- ✅ Backend: Sessions already persist server-side with PTY
- ✅ Backend: 100KB scrollback buffer for replay on reconnect
- ✅ Backend: `GET /api/terminal/sessions` endpoint (just added)
- ❌ Frontend: Web terminal is buggy on mobile
- ❌ Native: No native terminal component

**Implementation Options:**

| Approach | Pros | Cons |
|----------|------|------|
| **A) WebView wrapper** | Reuse existing code | Current terminal is broken on mobile |
| **B) Fix web terminal + WebView** | One codebase | Still not truly native feel |
| **C) Native terminal view** | Best UX, proper touch | More work, platform-specific |

**Recommended:** Option C - Native terminal views

**Reference Projects:**
- iOS: [Blink Shell](https://github.com/blinksh/blink) - Swift, professional terminal with Mosh
- iOS: [Pisth](https://github.com/ColdGrub1384/Pisth) - Simpler SSH client
- Android: [ConnectBot](https://github.com/connectbot/connectbot) - Mature Java/Kotlin terminal
- Android: [Termux](https://github.com/termux/termux-app) - Full Linux environment

**Architecture:**
```
React Native App
├── iOS Native Module (Swift)
│   └── TerminalView (inspired by Blink Shell)
├── Android Native Module (Kotlin)
│   └── TerminalView (inspired by ConnectBot)
└── JS Layer
    └── WebSocket connection to /api/terminal/{repoId}
```

**API Requirements:**
- ✅ `GET /api/terminal/sessions` - List all sessions
- ✅ `DELETE /api/terminal/sessions/{id}` - Close session
- ✅ `WS /api/terminal/{repoId}?session={id}` - Connect to session
- ⚠️ Increase session timeout from 30min to 4hr for mobile use

---

### 2. Claude Code Permission Bridge

**Goal:** Approve/deny Claude Code permission requests from mobile

**Current State:**
- ❌ No integration between Homeport and Claude Code
- ❓ Need to understand Claude Code's permission model

**Questions to Answer:**
1. Does Claude Code have hooks for permission requests?
2. Is there an API or is it purely stdin/stdout?
3. Can we intercept the permission prompt?

**Proposed Architecture:**
```
┌─────────────────────────────────────────────────────────┐
│                    Mobile App                           │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Permission Queue UI                              │   │
│  │ ┌─────────────────────────────────────────────┐ │   │
│  │ │ Claude wants to run: npm install            │ │   │
│  │ │ [Approve] [Deny] [View Details]             │ │   │
│  │ └─────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────┘   │
└───────────────────────┬─────────────────────────────────┘
                        │ WebSocket + Push Notifications
                        ▼
┌─────────────────────────────────────────────────────────┐
│                 Homeport Daemon                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Claude Bridge Service (NEW)                      │   │
│  │ - Maintains permission request queue             │   │
│  │ - Sends push notifications via FCM/APNs          │   │
│  │ - Responds to Claude Code with decisions         │   │
│  └─────────────────────────────────────────────────┘   │
└───────────────────────┬─────────────────────────────────┘
                        │ IPC / PTY interception
                        ▼
┌─────────────────────────────────────────────────────────┐
│                 Claude Code CLI                         │
│  - Running in terminal session                          │
│  - Permission prompts intercepted by bridge             │
└─────────────────────────────────────────────────────────┘
```

**New API Endpoints Needed:**
```
GET  /api/claude/sessions              - List Claude Code sessions
GET  /api/claude/sessions/{id}/pending - Get pending permission requests
POST /api/claude/sessions/{id}/approve - Approve a request
POST /api/claude/sessions/{id}/deny    - Deny a request
WS   /api/claude/events                - Real-time permission notifications
```

**Push Notification Payload:**
```json
{
  "type": "permission_request",
  "session_id": "abc123",
  "request_id": "req456",
  "action": "bash",
  "command": "npm install",
  "repo": "my-project",
  "timestamp": 1705512345
}
```

---

### 3. Port Preview

**Goal:** Preview dev servers Claude opened

**Current State:**
- ✅ Backend: Port proxy works (`/{port}/*`)
- ✅ Backend: Share modes (private/password/public)
- ✅ API: `GET /api/ports` lists all ports

**Implementation:**
Simple WebView pointing to `https://{port}.homeport.dev` or `https://homeport.dev/{port}/`

**UI:**
```
┌─────────────────────────────────────┐
│ Port 3000 - my-project              │
│ ┌─────────────────────────────────┐ │
│ │                                 │ │
│ │    [WebView of running app]    │ │
│ │                                 │ │
│ └─────────────────────────────────┘ │
│ [Share] [Open in Browser] [Close]   │
└─────────────────────────────────────┘
```

---

### 4. Push Notifications

**Goal:** Get notified when Claude needs attention

**Implementation:**
- iOS: Apple Push Notification Service (APNs)
- Android: Firebase Cloud Messaging (FCM)

**Notification Types:**
1. Permission request from Claude Code
2. Process crashed/exited
3. Port became available
4. Session idle warning (before auto-close)

**Backend Changes:**
- Store device tokens in SQLite
- New service for sending push notifications
- Rate limiting to prevent notification spam

---

### 5. Deep Linking

**Goal:** Tap notification → go directly to relevant screen

**URL Scheme:** `homeport://`

**Routes:**
```
homeport://terminal/{repoId}?session={sessionId}
homeport://approve/{sessionId}/{requestId}
homeport://port/{portNumber}
homeport://dashboard
```

---

## Tech Stack Recommendation

### React Native + Native Modules

**Why React Native:**
- Existing UI is React - shared mental model
- Reuse TypeScript types from `ui/src/lib/api.ts`
- Single codebase for iOS + Android (except terminal)
- Large ecosystem, good tooling

**Why Native Modules for Terminal:**
- React Native terminal libs are inadequate for real PTY
- Need proper VT100/xterm escape sequence handling
- Touch gestures, keyboard handling must be native

**Framework:** Expo (managed workflow initially, eject if needed for native modules)

**Key Libraries:**
- Navigation: React Navigation
- State: Zustand or Redux Toolkit
- Push: Expo Notifications or react-native-firebase
- WebSocket: Built-in or socket.io-client

---

## Project Structure

```
homeport-mobile/
├── app/                      # Expo Router screens
│   ├── (tabs)/
│   │   ├── index.tsx         # Dashboard
│   │   ├── terminals.tsx     # Terminal sessions list
│   │   └── settings.tsx      # Settings
│   ├── terminal/[id].tsx     # Terminal view
│   ├── port/[port].tsx       # Port preview (WebView)
│   └── approve/[id].tsx      # Permission approval
├── components/
│   ├── TerminalView/         # Native terminal component
│   ├── SessionCard.tsx
│   └── PortPreview.tsx
├── lib/
│   ├── api.ts                # Copy from web UI
│   ├── websocket.ts
│   └── notifications.ts
├── native/
│   ├── ios/
│   │   └── TerminalView.swift
│   └── android/
│       └── TerminalView.kt
└── app.json
```

---

## API Improvements Needed

### For Better Mobile Experience

1. **Batch endpoint** - Reduce round trips
   ```
   GET /api/dashboard
   Returns: { repos, ports, sessions, processes, activity }
   ```

2. **WebSocket for real-time updates**
   ```
   WS /api/events
   Events: port_change, session_change, process_change, permission_request
   ```

3. **Pagination** - For large repo/session lists
   ```
   GET /api/repos?limit=20&offset=0
   ```

4. **ETag/caching headers** - Reduce bandwidth

5. **Increase session timeout** - 30min → 4hr for mobile

---

## Development Phases

### Phase 1: Foundation (MVP)
- [ ] Scaffold Expo project
- [ ] Implement API client (copy from web)
- [ ] Dashboard screen showing repos, ports, sessions
- [ ] WebView port preview
- [ ] Basic auth (session cookie)

### Phase 2: Terminal
- [ ] Research Blink Shell / ConnectBot implementations
- [ ] Create native iOS terminal module
- [ ] Create native Android terminal module
- [ ] Connect to existing WebSocket API
- [ ] Session reconnection with scrollback replay

### Phase 3: Claude Code Integration
- [ ] Research Claude Code permission model
- [ ] Design Claude Bridge service
- [ ] Implement permission queue API
- [ ] Build approval UI
- [ ] Add WebSocket for real-time permissions

### Phase 4: Notifications
- [ ] Set up FCM/APNs
- [ ] Implement device token registration
- [ ] Add push notification triggers to daemon
- [ ] Deep linking from notifications

### Phase 5: Polish
- [ ] Offline support / caching
- [ ] Biometric auth
- [ ] Widget for quick actions (iOS/Android)
- [ ] Apple Watch companion (stretch)

---

## Open Questions

1. **Claude Code Integration:** How does Claude Code handle permission prompts? Is there a hook system or API we can integrate with?

2. **Terminal Native Modules:** Build from scratch or find existing React Native bridge to Blink/ConnectBot code?

3. **Authentication:** Use existing session cookie or implement token-based auth for mobile?

4. **Offline Mode:** How much functionality should work offline? Cache last-known state?

5. **Distribution:** TestFlight + Play Store internal testing initially, or just side-load?

---

## Resources

- [Blink Shell Source](https://github.com/blinksh/blink)
- [ConnectBot Source](https://github.com/connectbot/connectbot)
- [Termux Source](https://github.com/termux/termux-app)
- [Expo Documentation](https://docs.expo.dev/)
- [React Native WebSocket](https://reactnative.dev/docs/network#websocket-support)
