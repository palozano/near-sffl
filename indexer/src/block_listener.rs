use futures::future::join_all;
use near_indexer::near_primitives::views::ReceiptView;
use near_indexer::near_primitives::{
    types::{AccountId, TransactionOrReceiptId},
    views::{ActionView, ReceiptEnumView},
};
use tokio::sync::mpsc;

use crate::errors::Result;

#[derive(Clone, Debug)]
pub(crate) struct CandidateData {
    pub transaction_or_receipt_id: TransactionOrReceiptId,
    pub payloads: Vec<Vec<u8>>,
}

pub(crate) struct BlockListener {
    stream: mpsc::Receiver<near_indexer::StreamerMessage>,
    receipt_sender: mpsc::Sender<CandidateData>,
    da_contract_id: AccountId,
}

impl BlockListener {
    pub(crate) fn new(
        stream: mpsc::Receiver<near_indexer::StreamerMessage>,
        receipt_sender: mpsc::Sender<CandidateData>,
        da_contract_id: AccountId,
    ) -> Self {
        Self {
            stream,
            receipt_sender,
            da_contract_id,
        }
    }

    fn receipt_filer_map(da_contract_id: &AccountId, receipt: ReceiptView) -> Option<CandidateData> {
        if &receipt.receiver_id != da_contract_id {
            return None;
        }

        let actions = if let ReceiptEnumView::Action { actions, .. } = receipt.receipt {
            actions
        } else {
            return None;
        };

        let payloads = actions
            .into_iter()
            .filter_map(|el| match el {
                ActionView::FunctionCall { method_name, args, .. } if method_name == "submit" => Some(args.into()),
                _ => None,
            })
            .collect::<Vec<Vec<u8>>>();

        if payloads.is_empty() {
            return None;
        }

        Some(CandidateData {
            transaction_or_receipt_id: TransactionOrReceiptId::Receipt {
                receipt_id: receipt.receipt_id,
                receiver_id: receipt.receiver_id,
            },
            payloads,
        })
    }

    pub(crate) async fn start(self) -> Result<()> {
        let Self {
            mut stream,
            receipt_sender,
            da_contract_id,
        } = self;

        while let Some(streamer_message) = stream.recv().await {
            let candidates_data: Vec<CandidateData> = streamer_message
                .shards
                .into_iter()
                .flat_map(|shard| shard.chunk)
                .flat_map(|chunk| {
                    chunk
                        .receipts
                        .into_iter()
                        .filter_map(|receipt| Self::receipt_filer_map(&da_contract_id, receipt))
                })
                .collect();

            if candidates_data.is_empty() {
                continue;
            }

            let results = join_all(candidates_data.into_iter().map(|receipt| receipt_sender.send(receipt))).await;

            results.into_iter().collect::<Result<_, _>>()?;
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use crate::block_listener::{BlockListener, CandidateData};
    use near_crypto::{KeyType, PublicKey};
    use near_indexer::near_primitives::hash::CryptoHash;
    use near_indexer::near_primitives::types::{AccountId, TransactionOrReceiptId};
    use near_indexer::near_primitives::views::{ActionView, ReceiptEnumView, ReceiptView};
    use std::str::FromStr;

    impl PartialEq for CandidateData {
        fn eq(&self, other: &Self) -> bool {
            let res = match (&self.transaction_or_receipt_id, &other.transaction_or_receipt_id) {
                (
                    TransactionOrReceiptId::Receipt {
                        receiver_id,
                        receipt_id,
                    },
                    TransactionOrReceiptId::Receipt {
                        receipt_id: other_receipt_id,
                        receiver_id: other_receiver_id,
                    },
                ) => return receipt_id == other_receipt_id && receiver_id == other_receiver_id,
                (
                    TransactionOrReceiptId::Transaction {
                        transaction_hash,
                        sender_id,
                    },
                    TransactionOrReceiptId::Transaction {
                        transaction_hash: other_hash,
                        sender_id: other_id,
                    },
                ) => return transaction_hash == other_hash && sender_id == other_id,
                _ => false,
            };

            self.payloads == other.payloads && res
        }
    }

    #[test]
    fn test_receipt_filter_map() {
        let da_contract_id = AccountId::from_str("a.test").unwrap();

        let common_action_receipt = ReceiptEnumView::Action {
            signer_id: AccountId::from_str("test_signer").unwrap(),
            signer_public_key: PublicKey::empty(KeyType::ED25519),
            gas_price: 100,
            output_data_receivers: vec![],
            input_data_ids: vec![CryptoHash::hash_bytes(b"test_input_data_id")],
            actions: vec![ActionView::FunctionCall {
                method_name: "submit".into(),
                args: vec![1, 2, 3].into(),
                gas: 100,
                deposit: 100,
            }],
        };

        let test_predecessor_id = AccountId::from_str("test_predecessor").unwrap();
        let valid_receipt = ReceiptView {
            predecessor_id: test_predecessor_id.clone(),
            receiver_id: da_contract_id.clone(),
            receipt_id: CryptoHash::hash_bytes(b"test_receipt_id"),
            receipt: common_action_receipt.clone(),
        };

        let invalid_receiver_receipt = ReceiptView {
            predecessor_id: test_predecessor_id.clone(),
            receiver_id: AccountId::from_str("other_contract").unwrap(),
            receipt_id: CryptoHash::hash_bytes(b"test_receipt_id"),
            receipt: common_action_receipt,
        };

        let invalid_action_receipt = ReceiptView {
            predecessor_id: test_predecessor_id.clone(),
            receiver_id: da_contract_id.clone(),
            receipt_id: CryptoHash::hash_bytes(b"test_receipt_id"),
            receipt: ReceiptEnumView::Data {
                data_id: CryptoHash::hash_bytes(b"test_data_id"),
                data: Some(vec![1, 2, 3]),
            },
        };

        // Test valid receipt
        assert_eq!(
            BlockListener::receipt_filer_map(&da_contract_id, valid_receipt.clone()),
            Some(CandidateData {
                transaction_or_receipt_id: TransactionOrReceiptId::Receipt {
                    receipt_id: valid_receipt.receipt_id.clone(),
                    receiver_id: valid_receipt.receiver_id.clone(),
                },
                payloads: vec![vec![1, 2, 3]],
            })
        );

        // Test invalid receiver receipt
        assert_eq!(
            BlockListener::receipt_filer_map(&da_contract_id, invalid_receiver_receipt),
            None
        );

        // Test invalid action receipt
        assert_eq!(
            BlockListener::receipt_filer_map(&da_contract_id, invalid_action_receipt),
            None
        );
    }

    #[test]
    fn test_multiple_submit_actions() {
        let da_contract_id = AccountId::from_str("a.test").unwrap();

        let common_action_receipt = ReceiptEnumView::Action {
            signer_id: AccountId::from_str("test_signer").unwrap(),
            signer_public_key: PublicKey::empty(KeyType::ED25519),
            gas_price: 100,
            output_data_receivers: vec![],
            input_data_ids: vec![CryptoHash::hash_bytes(b"test_input_data_id")],
            actions: vec![
                ActionView::FunctionCall {
                    method_name: "submit".into(),
                    args: vec![1, 2, 3].into(),
                    gas: 100,
                    deposit: 100,
                },
                ActionView::FunctionCall {
                    method_name: "submit".into(),
                    args: vec![4, 4, 4].into(),
                    gas: 100,
                    deposit: 100,
                },
                ActionView::FunctionCall {
                    method_name: "random".into(),
                    args: vec![1, 2, 3].into(),
                    gas: 100,
                    deposit: 100,
                },
                ActionView::DeleteAccount {
                    beneficiary_id: da_contract_id.clone()
                }
            ],
        };

        let test_predecessor_id = AccountId::from_str("test_predecessor").unwrap();
        let valid_receipt = ReceiptView {
            predecessor_id: test_predecessor_id.clone(),
            receiver_id: da_contract_id.clone(),
            receipt_id: CryptoHash::hash_bytes(b"test_receipt_id"),
            receipt: common_action_receipt.clone(),
        };

        // Test valid receipt
        assert_eq!(
            BlockListener::receipt_filer_map(&da_contract_id, valid_receipt.clone()),
            Some(CandidateData {
                transaction_or_receipt_id: TransactionOrReceiptId::Receipt {
                    receipt_id: valid_receipt.receipt_id.clone(),
                    receiver_id: valid_receipt.receiver_id.clone(),
                },
                payloads: vec![vec![1, 2, 3], vec![4, 4, 4]],
            })
        );
    }
}
