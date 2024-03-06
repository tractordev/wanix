import * as jazz from "./jazz.js";

export interface Todo {
  id: string;
  title: string;
  completed: boolean;
  creator: string;
}

export class Store {
  todos: Todo[];

  constructor(node, todos, username) {
    this.creator = username;
    this.todos = todos;
    jazz.autoSub(todos.id, node, (todos) => {
      this.todos = todos;
      console.log(todos, todos.length);
      this.map(e=>console.log(e));
      m.redraw();
    });
  }

  map(cb) {
    return Array.from(this.todos).filter((obj, index, self) =>
      obj && index === self.findIndex((t) => t.id === obj.id)
    ).map(cb);
  }

  addTodo(title: string) {
    this.todos.mutate(t =>{
      t.append({title, completed: false, creator: this.creator, id: Date.now().toString()});
    });
  }

  completeTodo(idx: number, b: boolean) {
    const todo = this.todos[idx];
    todo.completed = b;
    this.todos.mutate(t => {
      t.prepend(todo, idx);
      t.delete(idx+1);
    });
  }

  removeTodo(idx: number) {
    this.todos.mutate(t => {
      t.delete(idx);
    });
  }
}
