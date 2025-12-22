/*
 * Desi Runtime Channel Implementation
 * 
 * Bounded and unbounded channels for safe message passing between tasks.
 */

#ifndef DESI_CHANNEL_H
#define DESI_CHANNEL_H

#include <pthread.h>
#include <stdbool.h>
#include <stdint.h>

/*
 * DesiChannel is a thread-safe bounded queue for message passing.
 * 
 * For bounded channels:
 * - send() blocks when buffer is full
 * - recv() blocks when buffer is empty
 * 
 * For unbounded channels (capacity = 0):
 * - Uses a linked list that grows dynamically
 */
typedef struct {
    void** buffer;              /* Circular buffer of elements */
    int64_t capacity;           /* Max elements (0 = unbounded) */
    int64_t head;               /* Read position */
    int64_t tail;               /* Write position */
    int64_t count;              /* Current number of elements */
    
    pthread_mutex_t lock;       /* Protects all fields */
    pthread_cond_t not_full;    /* Signaled when space available */
    pthread_cond_t not_empty;   /* Signaled when data available */
    
    bool closed;                /* No more sends allowed */
    int senders;                /* Number of active senders */
    int receivers;              /* Number of active receivers */
} DesiChannel;

/*
 * ChannelSender is a handle for sending to a channel.
 * Multiple senders can exist for the same channel.
 */
typedef struct {
    DesiChannel* channel;
} ChannelSender;

/*
 * ChannelReceiver is a handle for receiving from a channel.
 * Only one receiver typically exists (single consumer pattern).
 */
typedef struct {
    DesiChannel* channel;
} ChannelReceiver;

/* Create a new bounded channel with the given capacity */
DesiChannel* channel_new(int64_t capacity);

/* Create sender/receiver pair from channel */
ChannelSender* channel_sender(DesiChannel* ch);
ChannelReceiver* channel_receiver(DesiChannel* ch);

/* Clone a sender (for multiple producers) */
ChannelSender* sender_clone(ChannelSender* s);

/* Send a value to the channel. Blocks if full. Returns false if closed. */
bool channel_send(ChannelSender* s, void* value);

/* Try to send without blocking. Returns false if full or closed. */
bool channel_try_send(ChannelSender* s, void* value);

/* Receive a value from the channel. Blocks if empty. Returns NULL if closed and empty. */
void* channel_recv(ChannelReceiver* r);

/* Try to receive without blocking. Returns NULL if empty. */
void* channel_try_recv(ChannelReceiver* r);

/* Close the channel (no more sends allowed) */
void channel_close(DesiChannel* ch);

/* Check if channel is closed */
bool channel_is_closed(DesiChannel* ch);

/* Drop a sender (decrements sender count, closes if last) */
void sender_drop(ChannelSender* s);

/* Drop a receiver */
void receiver_drop(ChannelReceiver* r);

/* Destroy a channel (frees all resources) */
void channel_destroy(DesiChannel* ch);

#endif /* DESI_CHANNEL_H */
