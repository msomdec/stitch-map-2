interface ErrorNotificationProps {
  message: string | null;
  onDismiss?: () => void;
}

export function ErrorNotification({ message, onDismiss }: ErrorNotificationProps) {
  if (!message) return null;

  return (
    <div className="notification is-danger">
      {onDismiss && <button className="delete" onClick={onDismiss}></button>}
      {message}
    </div>
  );
}
