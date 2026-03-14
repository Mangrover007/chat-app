<!-- Systems:

I wanna support a few things. First of all:

Servers
    Channels -> two types
    - text channel
    - voice / video channel

Flow of data:
Client sends an HTTP request to https://discord.com/
Request contains info:
    - Server ID
    - Channel ID
    - Payload

MASTER PLAN:
    Move everthing to kubernetes.
        Single Redis deployment working as pubsub broker.
        My application deployment.
 -->
