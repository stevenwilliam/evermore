DROP TABLE IF EXISTS delivery_line, delivery, credit_ledger, customer_package,
    payment_event, payment_proof, payment, order_line, payment_suffix_claim,
    customer_order, order_sequence CASCADE;
DROP FUNCTION IF EXISTS next_order_number(text, text);
