/*
 * Desi Runtime Channel Implementation
 * Cross-platform: Uses platform.h macros for POSIX/Windows.
 */

#include "channel.h"
#include <stdlib.h>
#include <stdio.h>

/*
 * Create a new bounded channel
 */
DesiChannel* channel_new(int64_t capacity) {
    DesiChannel* ch = malloc(sizeof(DesiChannel));
    if (!ch) return NULL;
    
    if (capacity > 0) {
        ch->buffer = malloc(sizeof(void*) * capacity);
        if (!ch->buffer) {
            free(ch);
            return NULL;
        }
    } else {
        ch->buffer = NULL;  /* Unbounded uses linked list (not yet implemented) */
        capacity = 1;       /* Minimum buffer for now */
        ch->buffer = malloc(sizeof(void*));
    }
    
    ch->capacity = capacity;
    ch->head = 0;
    ch->tail = 0;
    ch->count = 0;
    ch->closed = false;
    ch->senders = 0;
    ch->receivers = 0;
    
    DESI_MUTEX_INIT(ch->lock);
    DESI_COND_INIT(ch->not_full);
    DESI_COND_INIT(ch->not_empty);
    
    return ch;
}

/*
 * Create a sender handle
 */
ChannelSender* channel_sender(DesiChannel* ch) {
    if (!ch) return NULL;
    
    ChannelSender* s = malloc(sizeof(ChannelSender));
    if (!s) return NULL;
    
    DESI_MUTEX_LOCK(ch->lock);
    ch->senders++;
    DESI_MUTEX_UNLOCK(ch->lock);
    
    s->channel = ch;
    return s;
}

/*
 * Create a receiver handle
 */
ChannelReceiver* channel_receiver(DesiChannel* ch) {
    if (!ch) return NULL;
    
    ChannelReceiver* r = malloc(sizeof(ChannelReceiver));
    if (!r) return NULL;
    
    DESI_MUTEX_LOCK(ch->lock);
    ch->receivers++;
    DESI_MUTEX_UNLOCK(ch->lock);
    
    r->channel = ch;
    return r;
}

/*
 * Clone a sender
 */
ChannelSender* sender_clone(ChannelSender* s) {
    if (!s || !s->channel) return NULL;
    return channel_sender(s->channel);
}

/*
 * Send a value (blocking)
 */
bool channel_send(ChannelSender* s, void* value) {
    if (!s || !s->channel) return false;
    DesiChannel* ch = s->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    
    /* Wait while buffer is full and channel is open */
    while (ch->count >= ch->capacity && !ch->closed) {
        DESI_COND_WAIT(ch->not_full, ch->lock);
    }
    
    if (ch->closed) {
        DESI_MUTEX_UNLOCK(ch->lock);
        return false;
    }
    
    /* Add to circular buffer */
    ch->buffer[ch->tail] = value;
    ch->tail = (ch->tail + 1) % ch->capacity;
    ch->count++;
    
    DESI_COND_SIGNAL(ch->not_empty);
    DESI_MUTEX_UNLOCK(ch->lock);
    
    return true;
}

/*
 * Try to send without blocking
 */
bool channel_try_send(ChannelSender* s, void* value) {
    if (!s || !s->channel) return false;
    DesiChannel* ch = s->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    
    if (ch->closed || ch->count >= ch->capacity) {
        DESI_MUTEX_UNLOCK(ch->lock);
        return false;
    }
    
    ch->buffer[ch->tail] = value;
    ch->tail = (ch->tail + 1) % ch->capacity;
    ch->count++;
    
    DESI_COND_SIGNAL(ch->not_empty);
    DESI_MUTEX_UNLOCK(ch->lock);
    
    return true;
}

/*
 * Receive a value (blocking)
 */
void* channel_recv(ChannelReceiver* r) {
    if (!r || !r->channel) return NULL;
    DesiChannel* ch = r->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    
    /* Wait while buffer is empty and senders exist */
    while (ch->count == 0 && !ch->closed) {
        DESI_COND_WAIT(ch->not_empty, ch->lock);
    }
    
    /* If empty and closed, return NULL */
    if (ch->count == 0 && ch->closed) {
        DESI_MUTEX_UNLOCK(ch->lock);
        return NULL;
    }
    
    /* Remove from circular buffer */
    void* value = ch->buffer[ch->head];
    ch->head = (ch->head + 1) % ch->capacity;
    ch->count--;
    
    DESI_COND_SIGNAL(ch->not_full);
    DESI_MUTEX_UNLOCK(ch->lock);
    
    return value;
}

/*
 * Try to receive without blocking
 */
void* channel_try_recv(ChannelReceiver* r) {
    if (!r || !r->channel) return NULL;
    DesiChannel* ch = r->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    
    if (ch->count == 0) {
        DESI_MUTEX_UNLOCK(ch->lock);
        return NULL;
    }
    
    void* value = ch->buffer[ch->head];
    ch->head = (ch->head + 1) % ch->capacity;
    ch->count--;
    
    DESI_COND_SIGNAL(ch->not_full);
    DESI_MUTEX_UNLOCK(ch->lock);
    
    return value;
}

/*
 * Close the channel
 */
void channel_close(DesiChannel* ch) {
    if (!ch) return;
    
    DESI_MUTEX_LOCK(ch->lock);
    ch->closed = true;
    /* Wake up all waiters */
    DESI_COND_BROADCAST(ch->not_full);
    DESI_COND_BROADCAST(ch->not_empty);
    DESI_MUTEX_UNLOCK(ch->lock);
}

/*
 * Check if closed
 */
bool channel_is_closed(DesiChannel* ch) {
    if (!ch) return true;
    
    DESI_MUTEX_LOCK(ch->lock);
    bool closed = ch->closed;
    DESI_MUTEX_UNLOCK(ch->lock);
    
    return closed;
}

/*
 * Drop a sender
 */
void sender_drop(ChannelSender* s) {
    if (!s || !s->channel) return;
    DesiChannel* ch = s->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    ch->senders--;
    /* If last sender, close the channel */
    if (ch->senders == 0) {
        ch->closed = true;
        DESI_COND_BROADCAST(ch->not_empty);
    }
    DESI_MUTEX_UNLOCK(ch->lock);
    
    free(s);
}

/*
 * Drop a receiver
 */
void receiver_drop(ChannelReceiver* r) {
    if (!r || !r->channel) return;
    DesiChannel* ch = r->channel;
    
    DESI_MUTEX_LOCK(ch->lock);
    ch->receivers--;
    DESI_MUTEX_UNLOCK(ch->lock);
    
    free(r);
}

/*
 * Destroy a channel
 */
void channel_destroy(DesiChannel* ch) {
    if (!ch) return;
    
    DESI_MUTEX_DESTROY(ch->lock);
    DESI_COND_DESTROY(ch->not_full);
    DESI_COND_DESTROY(ch->not_empty);
    
    if (ch->buffer) {
        free(ch->buffer);
    }
    free(ch);
}
