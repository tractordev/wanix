
export interface Todo {
  id: string;
  title: string;
  completed: boolean;
  creator: string;
}

export class Store {
  todos: Todo[];

  constructor() {
    if (!window.collab.todos) {
      this.todos = window.collab.group.createList();
      window.collab.mutate(c => {
        c.set("todos", this.todos.id);
      });
    } else {
      this.todos = window.collab.todos;
    }
    window.localNode.query(this.todos.id, (todos) => {
      this.todos = todos;
      console.log(todos);
      m.redraw();
    });
  }

  map(cb) {
    return Array.from(this.todos).map(cb);
  }

  addTodo(title: string) {
    this.todos.mutate(t =>{
      t.append({title, completed: false, creator: window.account.profile.name, id: Date.now().toString()});
    });
  }

  completeTodo(idx: number, b: boolean) {
    const todo = this.todos[idx];
    todo.completed = b;
    this.todos.mutate(t => {
      t.prepend(todo, idx);
      //t.delete(idx+1);
    });
    console.log("mutated");
  }

  removeTodo(idx: number) {
    this.todos.mutate(t => {
      t.delete(idx);
    });
  }
}
