---
title: Notification Channels
impact: LOW-MEDIUM
impactDescription: correct signaling idiom
tags: channels, signals, struct, notification
---

## Notification Channels

**Impact: LOW-MEDIUM (correct signaling idiom)**

Use chan struct{} for notification-only channels. Bool channels are ambiguous (what does false mean?).

**Incorrect (what's wrong):**

```go
disconnectCh := make(chan bool) // What does false mean?
```

**Correct (what's right):**

```go
disconnectCh := make(chan struct{})
// Send: disconnectCh <- struct{}{}
// Close for broadcast: close(disconnectCh)
```
