# **Aurora SDK and API Guide**

Now, let's get familiar with Aurora's API, SDK and the tooling around them. The [API documentation](https://developers.hcnet.org/api/) is particularly useful and you will find yourself consulting it often.

Spend a few hours reading before getting your hands dirty writing code:

- Skim through the [developer documentation](https://developers.hcnet.org/docs/) in general. 
- Try to understand what an [Account](https://developers.hcnet.org/docs/glossary/accounts/) is.
- Try to understand what a [Ledger](https://developers.hcnet.org/docs/glossary/ledger/), [Transaction](https://developers.hcnet.org/docs/glossary/transactions/) and [Operation](https://developers.hcnet.org/docs/glossary/operations/) is and their hierarchical nature. Make sure you understand how sequence numbers work.
- Go through the different [Operation](https://developers.hcnet.org/docs/start/list-of-operations/) types. Take a look at the Go SDK machinery for the [Account Creation](https://godoc.org/github.com/shantanu-hashcash/go/txnbuild#CreateAccount) and [Payment](https://godoc.org/github.com/shantanu-hashcash/go/txnbuild#Payment) operations and read the documentation examples.
- You will use the Testnet network frequently during Aurora's development. Get familiar with [what it is and how it is useful](https://developers.hcnet.org/docs/glossary/testnet/). Try to understand what [Friendbot](https://github.com/shantanu-hashcash/go/tree/master/services/friendbot) is.
- Read Aurora's API [introduction](https://developers.hcnet.org/api/introduction/) and make sure you understand what's HAL and XDR. Also, make sure you understand how streaming responses work.
- Get familiar with Aurora's REST API endpoints. There are two type of endpoints:
    - **Querying Endpoints**. They give you information about the network status. The output is based on the information obtained from Core or derived from it. These endpoints refer to resources. These resources can:
        - Exist in the Hcnet network. Take a look at the [endpoints associated with each resource](https://developers.hcnet.org/api/resources/).
        - Are abstractions which don't exist in the Hcnet network but they are useful to the end user. Take a look at their [endpoints](https://developers.hcnet.org/api/aggregations/).
    - **Submission Endpoints**. There is only one, the [transaction submission endpoint](https://www.hcnet.org/developers/aurora/reference/endpoints/transactions-create.html). You will be using it explicitly next.