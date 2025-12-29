// Minimal EventToken.h for cross-compilation with MinGW
// This provides the EventRegistrationToken structure needed by WebView2.h

#ifndef _EVENTTOKEN_H_
#define _EVENTTOKEN_H_

#include <stdint.h>

typedef struct EventRegistrationToken {
    int64_t value;
} EventRegistrationToken;

#endif // _EVENTTOKEN_H_
