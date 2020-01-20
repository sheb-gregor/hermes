# Hermes

Event delivery service

## Communication protocol

### Client

- Establish WS connection using this url, where:
    - `<server_address>` — `ip:port`, `host:port` or `my.some.domain`
    - `<user_uid>` — any unique string for user identification, depends on the **UUID** format used by event producers

```
ws://<server_address>/_ws/subscribe?uuid=<user_uid>
```

> One unique user can establish many sessions from different clients. The service will deliver a `direct` event to each of them.
    
- Send handshake

```json
{
  "channel":"ws_status",
  "event": "handshake"
}
```

- Subscribe to event channel(s):

```json
{
  "channel":"ws_status",
  "event": "subscribe",
  "command": {
    "channel":"ctp.notifications.prices",
    "event": "ex_market_data"
  }
}
```

Instead channel name can be passed **`***`**. 
**`***`** is a reserved symbol for wildcard subscription.

Field `event` is optional and can be omitted.

— Server Ping

Server sends ping to all clients with some period:

```json
{
  "channel": "ws_status", 
  "event": "ping"
}
```
Client must send response:

```json
{
  "channel": "ws_status", 
  "event": "pong"
}
```

Otherwise connection will be dropped.


### Events

Event has next format:

```json
{
  "broadcast": true,
  "channel": "<channel>",
  "event": "<event>",
  "data": {
    "<event>": {}
  }
}
```

- `broadcast` - indicates that this is a broadcast event, otherwise it is direct to the user.
- `data` - this field contains a field whose name is equal to the value of the `event` field. 


Example:

```json
{
  "broadcast": true,
  "channel": "ctp.notifications.prices",
  "event": "ex_market_data",
  "data": {
    "ex_market_data": {
      "symbol": "BCHBTC",
      "price": 0.03828405,
      "volume": 42809.069,
      "source": "coingecko",
      "time": "2020-01-20T08:28:40.68014Z"
    }
  }
}
```

