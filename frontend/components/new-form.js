import { Modal } from "antd"

function NewForm({ visible, onNewOk, onNewCancel }) {
  return (
    <Modal
      title='Basic Modal'
      visible={visible}
      onOk={onNewOk}
      onCancel={onNewCancel}
    >
      <p>Some contents...</p>
    </Modal>
  )
}

export default NewForm
