
export interface Todo {
  title: string;
  completed: boolean;
}

export class Store {
  todos: Todo[];

  constructor() {
    const storedTodos = localStorage.getItem("todos");
    if (storedTodos) {
      this.todos = JSON.parse(storedTodos);
    } else {
      this.todos = [];
    }
  }

  save() {
    localStorage.setItem("todos", JSON.stringify(this.todos));
  }

  addTodo(title: string) {
    this.todos.push({title, completed: false});
    this.save();
  }

  completeTodo(idx: number, b: boolean) {
    this.todos[idx].completed = b;
    this.save();
  }

  removeTodo(idx: number) {
    this.todos.splice(idx, 1);
    this.save();
  }
}
