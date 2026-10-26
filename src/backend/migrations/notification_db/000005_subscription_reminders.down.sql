-- Durable identities/outcomes must not disappear on an ordinary rollback.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM notification_subscription_inbox) OR
    EXISTS (SELECT 1 FROM notification_subscription_projection) OR
    EXISTS (SELECT 1 FROM notification_reminder_dispatch) THEN
   RAISE EXCEPTION 'subscription reminder evidence exists';
 END IF;
END $$;
DROP TABLE notification_reminder_dispatch;
DROP TABLE notification_subscription_projection;
DROP TABLE notification_subscription_inbox;
