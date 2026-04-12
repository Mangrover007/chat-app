import aiohttp

import uuid, json, random, string, time, os
import threading
# import base64

# from dateutil import parser

import asyncio

from websockets.asyncio.client import connect
from websockets.asyncio.client import ClientConnection

# set websockets library logs to DEBUG level
# import logging
# logger = logging.getLogger('websockets')
# logger.setLevel(logging.DEBUG)
# logger.addHandler(logging.StreamHandler())

CLIENT_NUM = 100
MSG_NUM = 100
TERM_NUM = 4
EXPECTED = CLIENT_NUM * MSG_NUM * TERM_NUM

MAX_QUEUE = 4096
GUILD_ID = 'b510ec76-2464-4d8e-9a6e-0c9a8560c635'   # guild_A
CHANNEL_ID = 'be30d8bf-0897-4d3a-abb4-96b5f28aa5fb' # general

IP = '10.111.94.59'

# intercept client send messages to catch and log everything
class Recorder:
    def __init__(self):
        # message sent by each client in order
        self.msgs = [None] * EXPECTED
        self.msg_cnt = 0

    def Record_msg(self, _: dict):
        # self.msgs.append({
        #     'sender': msg.get('Sender') or msg.get('UserID'),
        #     'content': msg.get('Content'),
        #     'timestamp': parser.parse(msg.get('Timestamp')).timestamp(),
        # })
        self.msgs[self.msg_cnt] = 1
        self.msg_cnt += 1

class Client(Recorder):
    ws_url = f'ws://{IP}:8080/ws/'
    http_url = f'http://{IP}:8080'

    def __init__(self):
        super().__init__()
        self.session: aiohttp.ClientSession

        self.username = uuid.uuid4()
        self.uid: str

        self.stop = threading.Event()
        self.ws: ClientConnection | None

        # these two lists to track per client message send and recv
        # self.msg_sent = []
        # self.msg_recv = []

        print(f'INFO: new CLIENT ID: {self.username}')

    async def _register(self) -> str:
        async with self.session.post(f'{self.http_url}/api/register', json={
            'Username': str(self.username),
            'Password': 'mango',
        }) as res:
            body = await res.read()
            print(body, res.status)
            data = json.loads(body)
            assert isinstance(data, dict)
            return data.get('id')

    async def _join_guild(self, guild_id: str):
        await self.session.post(f'{self.http_url}/api/{guild_id}', headers={
            'x-uid': self.uid,
        })

    async def _login_guild(self, guild_id: str, channel_id: str):
        await self.session.get(f'{self.ws_url}/{guild_id}/{channel_id}', headers={
            'x-uid': self.uid,
        })

    async def _send_msgs(self, guild_id: str, channel_id: str):
        for _ in range(0, MSG_NUM):
            msg = ''.join(random.choices(string.ascii_letters + string.digits, k=10))
            async with self.session.post(f'{self.http_url}/api/{guild_id}/{channel_id}', json={
                'username': str(self.username),
                'content': msg,
            }, headers={
                'x-uid': self.uid,
                # 'content-type': 'application/json',
            }) as res:
                if res.status == 200:
                    pass
                    # body = json.loads(await res.read())
                    # assert isinstance(body, dict)
                    # self.msg_sent.append(body)
                else:
                    print(f'ERROR: could not send message: {res.status}')
                    print(res.headers.get('Location'))
                    os._exit(1)

    async def _recv_msgs(self):
        if self.ws is None:
            print('ERROR: could not establish websocket connection')
            os._exit(1)
        ec = 0
        while self.msg_cnt < EXPECTED - 1:
            print(self.msg_cnt)
            try:
                _ = await self.ws.recv()
                # msg = await self.ws.recv()
                # self.Record_msg(json.loads(msg))
                self.Record_msg({})
                # self.msg_recv.append(json.loads(msg))
            except Exception as e:
                print('ERROR (_recv_msgs):', e, self.msg_cnt)
                if ec >= 10:
                    break
                ec += 1
                # break
                # os._exit(1)


    async def perform(self, guild_id: str, channel_id: str):
        self.session = aiohttp.ClientSession(self.http_url)
        self.uid = await self._register()
        async with connect(self.ws_url, additional_headers={
            'x-uid': self.uid,
        }, close_timeout=7, max_queue=MAX_QUEUE) as ws:
            # def _ping_handler(payload):
            #     print('PING RECEIVED, SENDING PONG', payload)
            # ws.pong_handler = _ping_handler
            self.ws = ws
            await self._join_guild(guild_id=guild_id)
            await self._login_guild(guild_id=guild_id, channel_id=channel_id)

            print('SLEEPING FOR 5 SECONDS before starting to send msgs')
            await asyncio.sleep(5)
            print('SENDING MESSAGES')

            t1 = asyncio.create_task(self._send_msgs(guild_id, channel_id))
            t2 = asyncio.create_task(self._recv_msgs())

            await asyncio.gather(t1, t2)
            await self.session.close()

async def main():
    tasks: list[asyncio.Task] = []
    clients: list[Client] = []
    for _ in range(0, CLIENT_NUM):
        c = Client()
        clients.append(c)
        t = asyncio.create_task(c.perform(guild_id=GUILD_ID, channel_id=CHANNEL_ID))
        tasks.append(t)

    print('All clients have been initialized.')
    input('Press ENTER to start sending messages: ')
    start_time = time.perf_counter()

    await asyncio.gather(*tasks)

    print('ALL clients have sent all messages. Waiting for all receiving \
tasks to finish.')

    print('ALL receiving tasks are finished.')
    end_time = time.perf_counter()

    # base64_dumps = set()
    # for c in clients:
    #     c.msgs.sort(key=lambda x: x['timestamp'])

    # with open('tests/dump', 'w') as dump:
    #     for c in clients:
    #         print(f'Client: {c.uid}, messages received count: {len(c.msgs)}')
    #         jsondump = json.dumps(c.msgs, indent=2)
    #         dump.write(jsondump)
    #         hashed = base64.b64encode(jsondump.encode()).decode()
    #         base64_dumps.add(hashed)

    # failed = False
    # with open('tests/dump', 'w') as dump:
    #     for i in range(0, MSG_NUM * CLIENT_NUM):
    #         msgs = set()
    #         state = {}
    #         for c in clients:
    #             msgs.add(tuple(c.msgs[i].items()))
    #             state[c.uid] = c.msgs[i]
    #         if len(msgs) != 1:
    #             print('FAILED MESSAGE: ', msgs)
    #             # print('STATE: ', state)
    #             dump.write(json.dumps(state, indent=2))
    #             failed = True
    #             break
    #         else:
    #             # print(f'The {i}-th message of all clients is the same')
    #             pass

    # print(len(base64_dumps))
    # if len(base64_dumps) == 1:
#     if failed == False:
#         print('TEST PASSED: All clients received the same messages \
# in the same order.')
#     else:
#         print('TEST FAILED: Clients did not receive the same messages \
# in the same order.')

    for c in clients:
        print(f'MESSAGES: {len(c.msgs)}')
    print(f'Time taken: {end_time - 5 - start_time:.4} seconds')

if __name__ == '__main__':
    asyncio.run(main())
